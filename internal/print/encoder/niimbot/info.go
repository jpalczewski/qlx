package niimbot

import (
	"encoding/binary"
	"fmt"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

const (
	cmdGetInfo   = 0x40
	cmdRfidInfo  = 0x1A
	cmdHeartbeat = 0xDC
)

// Ensure NiimbotEncoder implements StatusQuerier.
var _ encoder.StatusQuerier = (*NiimbotEncoder)(nil)

// Connect sends the initial handshake packet (0xC1).
func (e *NiimbotEncoder) Connect(tr transport.Transport) error {
	return e.transceive(tr, cmdConnect, []byte{0x01}, respOffsetStandard)
}

// Heartbeat sends 0xDC and parses the response.
// Response type varies: 0xDD (advanced1), 0xDE (basic), 0xD9 (advanced2).
func (e *NiimbotEncoder) Heartbeat(tr transport.Transport) (encoder.HeartbeatResult, error) {
	resp, err := e.transceiveAnyResponse(tr, cmdHeartbeat, []byte{0x01})
	if err != nil {
		return encoder.HeartbeatResult{}, fmt.Errorf("heartbeat: %w", err)
	}

	result := encoder.HeartbeatResult{Battery: -1}
	data := resp.Data

	switch len(data) {
	case 10:
		// D110 format: skip 8, lidClosed, chargeLevel
		result.LidClosed = data[8] == 0
		result.Battery = int(data[9])
	case 13:
		// B1 format: skip 9, lidClosed, chargeLevel, paperInserted, rfidSuccess
		result.LidClosed = data[9] == 0
		result.Battery = int(data[10])
		result.PaperLoaded = data[11] == 0
	case 19:
		// Extended: skip 15, lidClosed, chargeLevel, paperInserted, rfidSuccess
		result.LidClosed = data[15] == 0
		result.Battery = int(data[16])
		result.PaperLoaded = data[17] == 0
	case 20:
		// Extra extended: skip 18, paperInserted, rfidSuccess
		result.PaperLoaded = data[18] == 0
	default:
		// Unknown format, return what we have
	}

	return result, nil
}

// RfidInfo reads tape/label RFID tag info.
func (e *NiimbotEncoder) RfidInfo(tr transport.Transport) (encoder.RfidResult, error) {
	resp, err := e.transceiveWithResponse(tr, cmdRfidInfo, []byte{0x01}, respOffsetStandard)
	if err != nil {
		return encoder.RfidResult{TotalLabels: -1, UsedLabels: -1}, fmt.Errorf("rfid: %w", err)
	}

	result := encoder.RfidResult{TotalLabels: -1, UsedLabels: -1}
	data := resp.Data

	if len(data) < 2 || data[0] == 0 {
		// No RFID tag present
		return result, nil
	}

	// Parse: uuid(8), barcodeLen(1), barcode(N), serialLen(1), serial(N), totalPaper(u16), usedPaper(u16), type(u8)
	idx := 8 // skip uuid
	if idx >= len(data) {
		return result, nil
	}

	barcodeLen := int(data[idx])
	idx += 1 + barcodeLen // skip barcode

	if idx >= len(data) {
		return result, nil
	}
	serialLen := int(data[idx])
	idx += 1 + serialLen // skip serial

	if idx+5 > len(data) {
		return result, nil
	}
	result.TotalLabels = int(binary.BigEndian.Uint16(data[idx:]))
	result.UsedLabels = int(binary.BigEndian.Uint16(data[idx+2:]))
	labelType := data[idx+4]

	switch labelType {
	case 1:
		result.LabelType = "with-gaps"
	case 2:
		result.LabelType = "black"
	case 3:
		result.LabelType = "continuous"
	case 4:
		result.LabelType = "transparent"
	default:
		result.LabelType = fmt.Sprintf("unknown-%d", labelType)
	}

	return result, nil
}
