package niimbot

import (
	"image"
	"image/color"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// Helper: create mock response packet for a given command
func mockResponse(cmdType byte, respOffset byte, data []byte) []byte {
	pkt := Packet{Type: cmdType + respOffset, Data: data}
	return pkt.ToBytes()
}

// Build combined response data for a full B1 print flow of an image with `rows` rows
func buildMockResponses(rows int) []byte {
	var resp []byte
	resp = append(resp, mockResponse(0xC1, 1, []byte{0x01})...) // CONNECT
	resp = append(resp, mockResponse(0x21, 16, []byte{0x01})...) // SET_DENSITY
	resp = append(resp, mockResponse(0x23, 16, []byte{0x01})...) // SET_LABEL_TYPE
	resp = append(resp, mockResponse(0x01, 1, []byte{0x01})...)  // PRINT_START
	resp = append(resp, mockResponse(0x03, 1, []byte{0x01})...)  // PAGE_START
	resp = append(resp, mockResponse(0x13, 1, []byte{0x01})...)  // SET_PAGE_SIZE
	// No responses for bitmap rows
	resp = append(resp, mockResponse(0xE3, 1, []byte{0x01})...) // PAGE_END
	resp = append(resp, mockResponse(0xF3, 1, []byte{0x01})...) // PRINT_END (done=1)
	return resp
}

func TestNiimbotEncode_SendsDensity(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 384, 1))
	mock := &transport.MockTransport{}
	mock.SetReadData(buildMockResponses(1))

	enc := &NiimbotEncoder{}
	err := enc.Encode(img, "B1", encoder.PrintOpts{Density: 3}, mock)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// First packet should be CONNECT (0xC1), second should be SET_DENSITY (0x21)
	pkt, err := ParsePacket(mock.Written[:8])
	if err != nil {
		t.Fatalf("parse first packet: %v", err)
	}
	if pkt.Type != 0xC1 {
		t.Errorf("first packet type = %x, want 0xC1 (CONNECT)", pkt.Type)
	}

	// Find SET_DENSITY packet
	found := false
	for i := 0; i < len(mock.Written)-7; i++ {
		if mock.Written[i] == 0x55 && mock.Written[i+1] == 0x55 && mock.Written[i+2] == 0x21 {
			dpkt, derr := ParsePacket(mock.Written[i : i+8])
			if derr == nil && len(dpkt.Data) > 0 && dpkt.Data[0] == 3 {
				found = true
			}
			break
		}
	}
	if !found {
		t.Error("SET_DENSITY packet with density=3 not found")
	}
}

func TestNiimbotEncode_AllBlackRow(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 384, 1))
	for x := 0; x < 384; x++ {
		img.SetGray(x, 0, color.Gray{Y: 0}) // black
	}
	mock := &transport.MockTransport{}
	mock.SetReadData(buildMockResponses(1))

	enc := &NiimbotEncoder{}
	_ = enc.Encode(img, "B1", encoder.PrintOpts{Density: 3}, mock)

	// Find PRINT_BITMAP_ROW (0x85) in written data
	found := false
	for i := 0; i < len(mock.Written)-6; i++ {
		if mock.Written[i] == 0x55 && mock.Written[i+1] == 0x55 && mock.Written[i+2] == 0x85 {
			found = true
			break
		}
	}
	if !found {
		t.Error("PrintBitmapRow (0x85) packet not found")
	}
}

func TestNiimbotEncode_AllWhiteRow(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 384, 1))
	for x := 0; x < 384; x++ {
		img.SetGray(x, 0, color.Gray{Y: 255}) // white
	}
	mock := &transport.MockTransport{}
	mock.SetReadData(buildMockResponses(1))

	enc := &NiimbotEncoder{}
	_ = enc.Encode(img, "B1", encoder.PrintOpts{Density: 3}, mock)

	// Should use PRINT_EMPTY_ROW (0x84) for white rows
	found := false
	for i := 0; i < len(mock.Written)-6; i++ {
		if mock.Written[i] == 0x55 && mock.Written[i+1] == 0x55 && mock.Written[i+2] == 0x84 {
			found = true
			break
		}
	}
	if !found {
		t.Error("PrintEmptyRow (0x84) packet not found for all-white row")
	}
}
