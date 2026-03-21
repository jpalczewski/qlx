package niimbot

import (
	"testing"

	"github.com/erxyi/qlx/internal/print/transport"
)

func TestHeartbeat_B1(t *testing.T) {
	// 13-byte B1 heartbeat response (type 0xDD)
	// skip 9 bytes, lidClosed=0 (closed), charge=85, paper=0 (loaded), rfid=1
	respData := make([]byte, 13)
	respData[9] = 0x00  // lid closed
	respData[10] = 85   // battery
	respData[11] = 0x00 // paper loaded
	respData[12] = 0x01 // rfid ok

	mock := &transport.MockTransport{}
	pkt := Packet{Type: 0xDD, Data: respData}
	mock.SetReadData(pkt.ToBytes())

	enc := &NiimbotEncoder{}
	result, err := enc.Heartbeat(mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Battery != 85 {
		t.Errorf("battery=%d want 85", result.Battery)
	}
	if !result.LidClosed {
		t.Error("lid should be closed")
	}
	if !result.PaperLoaded {
		t.Error("paper should be loaded")
	}
}

func TestHeartbeat_D110(t *testing.T) {
	// 10-byte D110 heartbeat response (type 0xDD)
	// skip 8 bytes, lidClosed=0 (closed), chargeLevel=72
	respData := make([]byte, 10)
	respData[8] = 0x00 // lid closed
	respData[9] = 72   // battery

	mock := &transport.MockTransport{}
	pkt := Packet{Type: 0xDD, Data: respData}
	mock.SetReadData(pkt.ToBytes())

	enc := &NiimbotEncoder{}
	result, err := enc.Heartbeat(mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Battery != 72 {
		t.Errorf("battery=%d want 72", result.Battery)
	}
	if !result.LidClosed {
		t.Error("lid should be closed")
	}
}

func TestHeartbeat_LidOpen(t *testing.T) {
	// 13-byte B1 heartbeat response: lid open (data[9]=1)
	respData := make([]byte, 13)
	respData[9] = 0x01 // lid open
	respData[10] = 50  // battery
	respData[11] = 0x01 // paper not loaded

	mock := &transport.MockTransport{}
	pkt := Packet{Type: 0xDD, Data: respData}
	mock.SetReadData(pkt.ToBytes())

	enc := &NiimbotEncoder{}
	result, err := enc.Heartbeat(mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Battery != 50 {
		t.Errorf("battery=%d want 50", result.Battery)
	}
	if result.LidClosed {
		t.Error("lid should be open")
	}
	if result.PaperLoaded {
		t.Error("paper should not be loaded")
	}
}

func TestRfidInfo_NoTag(t *testing.T) {
	// Response with data[0]==0 means no RFID tag
	respData := []byte{0x00, 0x00}

	mock := &transport.MockTransport{}
	pkt := Packet{Type: cmdRfidInfo + respOffsetStandard, Data: respData}
	mock.SetReadData(pkt.ToBytes())

	enc := &NiimbotEncoder{}
	result, err := enc.RfidInfo(mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalLabels != -1 {
		t.Errorf("TotalLabels=%d want -1 (no tag)", result.TotalLabels)
	}
}

func TestRfidInfo_WithTag(t *testing.T) {
	// Build RFID response: uuid(8) + barcodeLen(1)+barcode(3) + serialLen(1)+serial(2) + total(u16) + used(u16) + type(u8)
	var respData []byte
	respData = append(respData, 0x01)           // data[0] != 0 → tag present
	respData = append(respData, make([]byte, 7)...) // rest of uuid (8 total: byte 0 already set, 7 more)
	// uuid is bytes 0..7, so we've written 8 bytes already (1 + 7)
	respData = append(respData, 3)              // barcodeLen=3
	respData = append(respData, 'A', 'B', 'C') // barcode
	respData = append(respData, 2)              // serialLen=2
	respData = append(respData, 'X', 'Y')       // serial
	respData = append(respData, 0x00, 0x64)     // totalLabels = 100
	respData = append(respData, 0x00, 0x0A)     // usedLabels = 10
	respData = append(respData, 0x01)           // labelType = with-gaps

	mock := &transport.MockTransport{}
	pkt := Packet{Type: cmdRfidInfo + respOffsetStandard, Data: respData}
	mock.SetReadData(pkt.ToBytes())

	enc := &NiimbotEncoder{}
	result, err := enc.RfidInfo(mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalLabels != 100 {
		t.Errorf("TotalLabels=%d want 100", result.TotalLabels)
	}
	if result.UsedLabels != 10 {
		t.Errorf("UsedLabels=%d want 10", result.UsedLabels)
	}
	if result.LabelType != "with-gaps" {
		t.Errorf("LabelType=%q want %q", result.LabelType, "with-gaps")
	}
}

func TestConnect_SendsHandshake(t *testing.T) {
	mock := &transport.MockTransport{}
	// Connect expects response type 0xC1 + 1 = 0xC2
	pkt := Packet{Type: cmdConnect + respOffsetStandard, Data: []byte{0x01}}
	mock.SetReadData(pkt.ToBytes())

	enc := &NiimbotEncoder{}
	err := enc.Connect(mock)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the written packet is CONNECT (0xC1)
	if len(mock.Written) < 7 {
		t.Fatalf("not enough bytes written: %d", len(mock.Written))
	}
	sent, err := ParsePacket(mock.Written)
	if err != nil {
		t.Fatalf("parse written packet: %v", err)
	}
	if sent.Type != cmdConnect {
		t.Errorf("sent type=0x%02x want 0x%02x", sent.Type, cmdConnect)
	}
}
