//go:build usb

package transport

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/gousb"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

const (
	// brotherVID is the USB Vendor ID for Brother Industries.
	brotherVID = 0x04F9

	// usbReadTimeout is the default timeout for USB bulk reads.
	usbReadTimeout = 2 * time.Second
)

// usbWriteTimeout bounds one bulk OUT transfer. It is generous so that a
// legitimately full printer buffer during a long job does not trip it, while a
// printer that stopped accepting data (for example in a "cannot feed" error)
// fails within this time instead of blocking forever. A var so tests can
// override it.
var usbWriteTimeout = 10 * time.Second

// GoUSBTransport provides bidirectional USB communication via gousb (libusb).
// Requires the "usb" build tag and libusb installed on the host system.
type GoUSBTransport struct {
	ctx   *gousb.Context
	dev   *gousb.Device
	cfg   *gousb.Config
	iface *gousb.Interface
	inEP  *gousb.InEndpoint
	outEP *gousb.OutEndpoint

	// closeCtx is cancelled by Close() to abort in-flight USB reads and writes
	// before releasing libusb resources. Prevents SIGSEGV on shutdown.
	closeCtx    context.Context
	closeCancel context.CancelFunc
}

// USBScanResult represents a discovered USB device.
type USBScanResult struct {
	Address string `json:"address"` // "VID:PID:Serial" e.g. "04F9:2042:000F1Z401370"
	Name    string `json:"name"`    // USB product string
	VID     uint16 `json:"vid"`
	PID     uint16 `json:"pid"`
	Serial  string `json:"serial"`
}

// Name returns the transport identifier.
func (t *GoUSBTransport) Name() string { return "gousb" }

// Open connects to a USB device identified by "VID:PID:Serial" address string.
func (t *GoUSBTransport) Open(ctx context.Context, address string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	vid, pid, serial, err := parseUSBAddress(address)
	if err != nil {
		return fmt.Errorf("gousb open: %w", err)
	}

	t.closeCtx, t.closeCancel = context.WithCancel(context.Background())
	t.ctx = gousb.NewContext()

	devs, err := t.ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid)
	})
	if err != nil {
		t.cleanup()
		return fmt.Errorf("gousb open devices: %w", err)
	}

	// Find the device matching the serial number.
	var matched *gousb.Device
	for _, d := range devs {
		s, sErr := d.SerialNumber()
		if sErr != nil {
			d.Close()
			continue
		}
		if s == serial {
			matched = d
		} else {
			d.Close()
		}
	}

	if matched == nil {
		t.cleanup()
		return fmt.Errorf("gousb: device %s not found", address)
	}

	t.dev = matched
	if autoDetachDefault(runtime.GOOS) {
		if err := t.dev.SetAutoDetach(true); err != nil {
			webutil.LogTrace("gousb: set auto detach: %v", err)
		}
	}

	// Use the active configuration rather than setting one (avoids kernel driver conflicts on macOS).
	cfgNum, err := t.dev.ActiveConfigNum()
	if err != nil {
		t.cleanup()
		return fmt.Errorf("gousb active config: %w", err)
	}
	t.cfg, err = t.dev.Config(cfgNum)
	if err != nil {
		t.cleanup()
		return fmt.Errorf("gousb config(%d): %w", cfgNum, err)
	}

	t.iface, err = t.cfg.Interface(0, 0)
	if err != nil {
		t.cleanup()
		// "bad access" (LIBUSB_ERROR_ACCESS) means another process already holds the interface
		// exclusively, or, on Linux, that a kernel driver or missing permissions are in the way.
		return fmt.Errorf("gousb interface: %w (another process may hold the printer, e.g. a CUPS queue "+
			"or a second QLX instance; on Linux check the usblp driver and udev permissions)", err)
	}

	// Find bulk IN and OUT endpoints.
	for _, ep := range t.iface.Setting.Endpoints {
		if ep.TransferType != gousb.TransferTypeBulk {
			continue
		}
		if ep.Direction == gousb.EndpointDirectionIn {
			t.inEP, err = t.iface.InEndpoint(ep.Number)
			if err != nil {
				t.cleanup()
				return fmt.Errorf("gousb in endpoint: %w", err)
			}
		} else {
			t.outEP, err = t.iface.OutEndpoint(ep.Number)
			if err != nil {
				t.cleanup()
				return fmt.Errorf("gousb out endpoint: %w", err)
			}
		}
	}

	if t.outEP == nil {
		t.cleanup()
		return errors.New("gousb: no bulk OUT endpoint found")
	}

	webutil.LogInfo("gousb: opened device %s", address)
	return nil
}

// autoDetachDefault reports whether the kernel driver should be detached automatically when
// claiming the printer interface for the given GOOS value (runtime.GOOS).
//
// Only Linux needs it: the usblp driver binds to every USB printer. On macOS the printer
// interface has no kernel driver, and libusb's detach path (device capture) needs root or a
// special Apple entitlement, so enabling it there turns a working unprivileged claim into
// "Can't detach kernel driver ... bad access". Without it a normal user can print on macOS.
func autoDetachDefault(goos string) bool {
	return goos == "linux"
}

// Write sends data to the USB bulk OUT endpoint. It fails with an error
// wrapping ErrTimeout if the printer does not accept the data within
// usbWriteTimeout, and returns early if Close() is called.
func (t *GoUSBTransport) Write(data []byte) (int, error) {
	if t.outEP == nil {
		return 0, errors.New("gousb: not connected")
	}
	ctx, cancel := ioContext(t.closeCtx, usbWriteTimeout)
	defer cancel()
	n, err := t.outEP.WriteContext(ctx, data)
	return n, classifyIOError("gousb write", ctx, usbWriteTimeout, err)
}

// Read reads data from the USB bulk IN endpoint. It fails with an error
// wrapping ErrTimeout after usbReadTimeout, and returns early on Close().
func (t *GoUSBTransport) Read(buf []byte) (int, error) {
	if t.inEP == nil {
		return 0, errors.New("gousb: no IN endpoint (write-only device)")
	}
	ctx, cancel := ioContext(t.closeCtx, usbReadTimeout)
	defer cancel()
	n, err := t.inEP.ReadContext(ctx, buf)
	return n, classifyIOError("gousb read", ctx, usbReadTimeout, err)
}

// Close cancels in-flight reads and releases all USB resources.
func (t *GoUSBTransport) Close() error {
	if t.closeCancel != nil {
		t.closeCancel() // cancel any in-flight ReadContext/WriteContext
	}
	// Brief yield so the cancelled transfer returns before we free libusb resources.
	time.Sleep(50 * time.Millisecond)
	t.cleanup()
	return nil
}

// cleanup releases resources in reverse order. Safe to call multiple times.
func (t *GoUSBTransport) cleanup() {
	if t.iface != nil {
		t.iface.Close()
		t.iface = nil
	}
	if t.cfg != nil {
		t.cfg.Close()
		t.cfg = nil
	}
	if t.dev != nil {
		t.dev.Close()
		t.dev = nil
	}
	if t.ctx != nil {
		t.ctx.Close()
		t.ctx = nil
	}
	t.inEP = nil
	t.outEP = nil
}

// ScanUSB enumerates connected Brother USB devices.
func ScanUSB() ([]USBScanResult, error) {
	ctx := gousb.NewContext()
	defer ctx.Close()

	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return desc.Vendor == gousb.ID(brotherVID)
	})
	if err != nil && len(devs) == 0 {
		return nil, fmt.Errorf("usb scan: %w", err)
	}

	var results []USBScanResult
	for _, d := range devs {
		product, _ := d.Product()
		serial, _ := d.SerialNumber()

		vid := uint16(d.Desc.Vendor)
		pid := uint16(d.Desc.Product)

		addr := fmt.Sprintf("%04X:%04X:%s", vid, pid, serial)
		results = append(results, USBScanResult{
			Address: addr,
			Name:    product,
			VID:     vid,
			PID:     pid,
			Serial:  serial,
		})

		d.Close()
	}

	webutil.LogInfo("usb scan: found %d Brother device(s)", len(results))
	return results, nil
}

// parseUSBAddress splits "VID:PID:Serial" into components.
func parseUSBAddress(addr string) (vid, pid uint16, serial string, err error) {
	parts := strings.SplitN(addr, ":", 3)
	if len(parts) < 3 {
		return 0, 0, "", fmt.Errorf("invalid USB address %q: expected VID:PID:Serial", addr)
	}

	v, err := strconv.ParseUint(parts[0], 16, 16)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid VID %q: %w", parts[0], err)
	}

	p, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid PID %q: %w", parts[1], err)
	}

	return uint16(v), uint16(p), parts[2], nil //nolint:gosec // G115: validated by ParseUint bounds
}

func init() {
	RegisterWithMeta("gousb", func() Transport { return &GoUSBTransport{} }, Meta{
		Label:              "USB (bidirectional)",
		AddressPlaceholder: "04F9:2042:SERIAL",
		Order:              10,
		Available:          true,
	})
}
