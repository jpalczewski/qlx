package niimbot

import (
	"bytes"
	"testing"
)

func TestPacketToBytes(t *testing.T) {
	pkt := Packet{Type: 0x21, Data: []byte{0x03}}
	got := pkt.ToBytes()
	// checksum: 0x21 ^ 0x01 ^ 0x03 = 0x23
	want := []byte{0x55, 0x55, 0x21, 0x01, 0x03, 0x23, 0xAA, 0xAA}
	if !bytes.Equal(got, want) {
		t.Errorf("ToBytes() = %x, want %x", got, want)
	}
}

func TestPacketToBytes_MultipleDataBytes(t *testing.T) {
	pkt := Packet{Type: 0x13, Data: []byte{0x00, 0xF0, 0x01, 0x80}}
	got := pkt.ToBytes()
	// checksum: 0x13 ^ 0x04 ^ 0x00 ^ 0xF0 ^ 0x01 ^ 0x80 = 0x66 (verify manually)
	if got[0] != 0x55 || got[1] != 0x55 {
		t.Error("bad header")
	}
	if got[len(got)-2] != 0xAA || got[len(got)-1] != 0xAA {
		t.Error("bad tail")
	}
	// Roundtrip
	parsed, err := ParsePacket(got)
	if err != nil {
		t.Fatalf("roundtrip parse error: %v", err)
	}
	if parsed.Type != 0x13 || !bytes.Equal(parsed.Data, pkt.Data) {
		t.Error("roundtrip mismatch")
	}
}

func TestPacketFromBytes(t *testing.T) {
	raw := []byte{0x55, 0x55, 0x21, 0x01, 0x03, 0x23, 0xAA, 0xAA}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatalf("ParsePacket() error = %v", err)
	}
	if pkt.Type != 0x21 {
		t.Errorf("Type = %x, want 0x21", pkt.Type)
	}
	if !bytes.Equal(pkt.Data, []byte{0x03}) {
		t.Errorf("Data = %x, want [03]", pkt.Data)
	}
}

func TestPacketBadChecksum(t *testing.T) {
	raw := []byte{0x55, 0x55, 0x21, 0x01, 0x03, 0xFF, 0xAA, 0xAA}
	_, err := ParsePacket(raw)
	if err == nil {
		t.Error("expected checksum error")
	}
}

func TestPacketEmptyData(t *testing.T) {
	pkt := Packet{Type: 0xDC, Data: []byte{}}
	got := pkt.ToBytes()
	parsed, err := ParsePacket(got)
	if err != nil {
		t.Fatalf("ParsePacket error: %v", err)
	}
	if parsed.Type != 0xDC || len(parsed.Data) != 0 {
		t.Error("empty data roundtrip failed")
	}
}
