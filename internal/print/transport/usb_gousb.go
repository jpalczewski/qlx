//go:build usb

package transport

import (
	"context"
	"errors"
	"fmt"
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

// GoUSBTransport provides bidirectional USB communication via gousb (libusb).
// Requires the "usb" build tag and libusb installed on the host system.
type GoUSBTransport struct {
	ctx   *gousb.Context
	dev   *gousb.Device
	cfg   *gousb.Config
	iface *gousb.Interface
	inEP  *gousb.InEndpoint
	outEP *gousb.OutEndpoint
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
	t.dev.SetAutoDetach(true)

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
		// On macOS, the Apple USB printer driver claims the interface.
		// Running with sudo or unloading the driver is required.
		return fmt.Errorf("gousb interface: %w (on macOS, try running with sudo or unload the kernel driver)", err)
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

// Write sends data to the USB bulk OUT endpoint.
func (t *GoUSBTransport) Write(data []byte) (int, error) {
	if t.outEP == nil {
		return 0, errors.New("gousb: not connected")
	}
	return t.outEP.Write(data)
}

// Read reads data from the USB bulk IN endpoint with a timeout.
func (t *GoUSBTransport) Read(buf []byte) (int, error) {
	if t.inEP == nil {
		return 0, errors.New("gousb: no IN endpoint (write-only device)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), usbReadTimeout)
	defer cancel()
	return t.inEP.ReadContext(ctx, buf)
}

// Close releases all USB resources.
func (t *GoUSBTransport) Close() error {
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
