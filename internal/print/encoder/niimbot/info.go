package niimbot

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

const (
	cmdGetInfo   = 0x40
	cmdRfidInfo  = 0x1A
	cmdHeartbeat = 0xDC
)

// Ensure NiimbotEncoder implements StatusQuerier.
var _ encoder.StatusQuerier = (*NiimbotEncoder)(nil)

// Connect sends the initial handshake packet (0xC1).
func (e *NiimbotEncoder) Connect(ctx context.Context, tr transport.Transport) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
func (e *NiimbotEncoder) RfidInfo(ctx context.Context, tr transport.Transport) (encoder.RfidResult, error) {
	if err := ctx.Err(); err != nil {
		return encoder.RfidResult{TotalLabels: -1, UsedLabels: -1}, err
	}
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
	idx++
	barcode := ""
	if idx+barcodeLen <= len(data) {
		barcode = string(data[idx : idx+barcodeLen])
		result.Barcode = barcode
	}
	idx += barcodeLen

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

	// Offline lookup for label dimensions by barcode
	if barcode != "" {
		w, h := lookupLabelSize(barcode)
		result.LabelWidthMm = w
		result.LabelHeightMm = h
	}

	return result, nil
}

// Known Niimbot label barcodes → dimensions (mm).
// Sourced from product listings and RFID tag data.
var knownLabels = map[string][2]int{
	// B1 / B21 common labels (50mm printhead)
	"6972842748577": {50, 30}, // T50*30-230
	"6972842748584": {40, 30}, // T40*30-230
	"6972842748591": {50, 80}, // T50*80-100
	"6972842748607": {30, 15}, // T30*15-400
	"6972842748614": {40, 60}, // T40*60-150
	"6972842748621": {50, 50}, // T50*50-150
	"6972842748638": {40, 40}, // T40*40-180
	"6972842748645": {25, 15}, // T25*15-400
	"6972842748652": {50, 25}, // T50*25-320
	"6972842748669": {40, 20}, // T40*20-320
	"6971501227927": {50, 20}, // 50x20-384
	// D11 / D110 labels (12mm printhead)
	"6972842748676": {12, 40}, // T12*40-160
	"6972842748683": {15, 30}, // T15*30-210
	"6972842748690": {14, 22}, // T14*22-260
}

// lookupLabelSize returns label dimensions for a known barcode.
func lookupLabelSize(barcode string) (widthMm, heightMm int) {
	if dims, ok := knownLabels[barcode]; ok {
		webutil.LogTrace("niimbot: barcode %s → %dx%d mm (offline db)", barcode, dims[0], dims[1])
		return dims[0], dims[1]
	}
	webutil.LogTrace("niimbot: barcode %s not in offline db", barcode)
	return 0, 0
}
