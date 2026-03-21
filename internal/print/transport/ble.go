//go:build ble

package transport

import (
	"errors"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var niimbotServiceUUID = bluetooth.NewUUID([16]byte{
	0xe7, 0x81, 0x0a, 0x71, 0x73, 0xae, 0x49, 0x9d,
	0x8c, 0x15, 0xfa, 0xa9, 0xae, 0xf0, 0xc3, 0xf2,
})

type BLETransport struct {
	adapter   *bluetooth.Adapter
	device    bluetooth.Device
	char      bluetooth.DeviceCharacteristic
	mu        sync.Mutex
	recvBuf   []byte
	connected bool
}

func (t *BLETransport) Name() string { return "ble" }

func (t *BLETransport) Open(address string) error {
	t.adapter = bluetooth.DefaultAdapter
	if err := t.adapter.Enable(); err != nil {
		return err
	}

	addr := bluetooth.Address{}
	addr.Set(address)

	device, err := t.adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}
	t.device = device

	services, err := device.DiscoverServices([]bluetooth.UUID{niimbotServiceUUID})
	if err != nil {
		device.Disconnect()
		return err
	}
	if len(services) == 0 {
		device.Disconnect()
		return errors.New("niimbot service not found")
	}

	chars, err := services[0].DiscoverCharacteristics(nil)
	if err != nil {
		device.Disconnect()
		return err
	}

	var found bool
	for _, c := range chars {
		// Find characteristic that supports write-without-response and notify
		t.char = c
		found = true
		break
	}
	if !found {
		device.Disconnect()
		return errors.New("suitable characteristic not found")
	}

	// Enable notifications
	err = t.char.EnableNotifications(func(buf []byte) {
		t.mu.Lock()
		t.recvBuf = append(t.recvBuf, buf...)
		t.mu.Unlock()
	})
	if err != nil {
		device.Disconnect()
		return err
	}

	t.connected = true
	return nil
}

func (t *BLETransport) Write(data []byte) (int, error) {
	if !t.connected {
		return 0, errors.New("BLE not connected")
	}
	_, err := t.char.WriteWithoutResponse(data)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (t *BLETransport) Read(buf []byte) (int, error) {
	if !t.connected {
		return 0, errors.New("BLE not connected")
	}
	// Poll for data with timeout
	for i := 0; i < 50; i++ {
		t.mu.Lock()
		if len(t.recvBuf) > 0 {
			n := copy(buf, t.recvBuf)
			t.recvBuf = t.recvBuf[n:]
			t.mu.Unlock()
			return n, nil
		}
		t.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	return 0, nil
}

func (t *BLETransport) Close() error {
	if t.connected {
		t.connected = false
		return t.device.Disconnect()
	}
	return nil
}

// BLEScanResult represents a discovered BLE device.
type BLEScanResult struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int    `json:"rssi"`
}

// ScanBLE scans for Niimbot BLE printers for 5 seconds.
func ScanBLE() ([]BLEScanResult, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, err
	}

	var results []BLEScanResult
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		time.Sleep(5 * time.Second)
		adapter.StopScan()
		close(done)
	}()

	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		name := device.LocalName()
		if name == "" {
			return
		}
		first := name[0]
		if first == 'B' || first == 'D' || first == 'A' || first == 'H' ||
			first == 'N' || first == 'C' || first == 'K' || first == 'S' ||
			first == 'P' || first == 'T' || first == 'M' || first == 'E' {
			mu.Lock()
			results = append(results, BLEScanResult{
				Address: device.Address.String(),
				Name:    name,
				RSSI:    int(device.RSSI),
			})
			mu.Unlock()
		}
	})

	<-done

	if err != nil {
		return results, err
	}
	return results, nil
}
