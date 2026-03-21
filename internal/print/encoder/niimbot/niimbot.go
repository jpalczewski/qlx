package niimbot

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

const (
	cmdConnect         = 0xC1
	cmdSetDensity      = 0x21
	cmdSetLabelType    = 0x23
	cmdPrintStart      = 0x01
	cmdPageStart       = 0x03
	cmdSetPageSize     = 0x13
	cmdPrintBitmapRow  = 0x85
	cmdPrintEmptyRow   = 0x84
	cmdPageEnd         = 0xE3
	cmdPrintEnd        = 0xF3
	cmdPrintStatus     = 0xA3

	respOffsetStandard = 1
	respOffsetDensity  = 16

	packetIntervalMs   = 10
)

// NiimbotEncoder implements the Encoder interface for Niimbot printers.
type NiimbotEncoder struct{}

// Name returns the encoder name.
func (e *NiimbotEncoder) Name() string { return "niimbot" }

// Models returns the list of supported models.
func (e *NiimbotEncoder) Models() []encoder.ModelInfo {
	infos := make([]encoder.ModelInfo, len(allModels))
	for i, m := range allModels {
		infos[i] = modelInfo(m)
	}
	return infos
}

// Encode executes the full B1 print protocol flow.
// Based on niimbluelib B1PrintTask: connect → init → page → image → end → poll status.
func (e *NiimbotEncoder) Encode(img image.Image, model string, opts encoder.PrintOpts, tr transport.Transport) error {
	var m niimbotModel
	found := false
	for _, candidate := range allModels {
		if candidate.ID == model {
			m = candidate
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown model: %s", model)
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	if width != m.PrintheadPx {
		return fmt.Errorf("image width %d does not match printhead width %d", width, m.PrintheadPx)
	}
	rows := bounds.Max.Y - bounds.Min.Y

	density := opts.Density
	if density < m.DensityMin || density > m.DensityMax {
		density = m.DensityDefault
	}

	// 0. CONNECT (initial negotiate, like niimbluelib)
	webutil.LogTrace("niimbot: === print start: model=%s rows=%d width=%d density=%d ===", model, rows, width, density)
	if err := e.transceive(tr, cmdConnect, []byte{0x01}, respOffsetStandard); err != nil {
		return fmt.Errorf("CONNECT: %w", err)
	}

	// 1. SET_DENSITY
	if err := e.transceive(tr, cmdSetDensity, []byte{byte(density)}, respOffsetDensity); err != nil {
		return fmt.Errorf("SET_DENSITY: %w", err)
	}

	// 2. SET_LABEL_TYPE (1 = gaps)
	if err := e.transceive(tr, cmdSetLabelType, []byte{0x01}, respOffsetDensity); err != nil {
		return fmt.Errorf("SET_LABEL_TYPE: %w", err)
	}

	// 3. PRINT_START (7-byte variant for B1)
	printStartData := make([]byte, 7)
	binary.BigEndian.PutUint16(printStartData[0:2], 1) // totalPages = 1
	// bytes 2-4: zeros
	printStartData[5] = 0x00 // pageColor
	printStartData[6] = 0x00
	if err := e.transceive(tr, cmdPrintStart, printStartData, respOffsetStandard); err != nil {
		return fmt.Errorf("PRINT_START: %w", err)
	}

	// 4. PAGE_START
	if err := e.transceive(tr, cmdPageStart, []byte{0x01}, respOffsetStandard); err != nil {
		return fmt.Errorf("PAGE_START: %w", err)
	}

	// 5. SET_PAGE_SIZE (6-byte: rows, cols, copies)
	pageSizeData := make([]byte, 6)
	binary.BigEndian.PutUint16(pageSizeData[0:2], uint16(rows))
	binary.BigEndian.PutUint16(pageSizeData[2:4], uint16(m.PrintheadPx))
	binary.BigEndian.PutUint16(pageSizeData[4:6], 1) // copies = 1
	if err := e.transceive(tr, cmdSetPageSize, pageSizeData, respOffsetStandard); err != nil {
		return fmt.Errorf("SET_PAGE_SIZE: %w", err)
	}

	// 6. Send image rows (fire-and-forget, with inter-packet delay)
	webutil.LogTrace("niimbot: sending %d image rows...", rows)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		rowNum := uint16(y - bounds.Min.Y)
		rowData := e.encodeRow(img, y, bounds.Min.X, m.PrintheadPx)

		time.Sleep(time.Duration(packetIntervalMs) * time.Millisecond)

		if isAllZero(rowData) {
			data := make([]byte, 3)
			binary.BigEndian.PutUint16(data[0:2], rowNum)
			data[2] = 1
			if err := e.sendOnly(tr, cmdPrintEmptyRow, data); err != nil {
				return fmt.Errorf("PRINT_EMPTY_ROW row %d: %w", rowNum, err)
			}
		} else {
			data := make([]byte, 2+3+1+len(rowData))
			binary.BigEndian.PutUint16(data[0:2], rowNum)
			data[2] = 0x00
			data[3] = 0x00
			data[4] = 0x00
			data[5] = 0x01
			copy(data[6:], rowData)
			if err := e.sendOnly(tr, cmdPrintBitmapRow, data); err != nil {
				return fmt.Errorf("PRINT_BITMAP_ROW row %d: %w", rowNum, err)
			}
		}
	}

	// 7. PAGE_END
	webutil.LogTrace("niimbot: image rows sent, sending PAGE_END")
	if err := e.transceive(tr, cmdPageEnd, []byte{0x01}, respOffsetStandard); err != nil {
		return fmt.Errorf("PAGE_END: %w", err)
	}

	// 8. Poll PRINT_END until done (like niimbluelib waitUntilPrintFinishedByPrintEndPoll)
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := e.transceiveWithResponse(tr, cmdPrintEnd, []byte{0x01}, respOffsetStandard)
		if err != nil {
			return fmt.Errorf("PRINT_END poll: %w", err)
		}
		if len(resp.Data) > 0 && resp.Data[0] == 1 {
			return nil // done!
		}
	}

	return fmt.Errorf("PRINT_END: printer did not finish after 10s")
}

// transceive sends a packet and reads+validates the response.
func (e *NiimbotEncoder) transceive(tr transport.Transport, cmdType byte, data []byte, respOffset byte) error {
	_, err := e.transceiveWithResponse(tr, cmdType, data, respOffset)
	return err
}

// transceiveWithResponse sends a packet and returns the parsed response packet.
// It reads packets from the transport, skipping unexpected ones (printer may send
// unsolicited packets like PrinterCheckLine 0xD3 or ResetTimeout 0xC6), until the
// expected response type arrives or max retries exceeded.
func (e *NiimbotEncoder) transceiveWithResponse(tr transport.Transport, cmdType byte, data []byte, respOffset byte) (Packet, error) {
	pkt := Packet{Type: cmdType, Data: data}
	if _, err := tr.Write(pkt.ToBytes()); err != nil {
		return Packet{}, fmt.Errorf("write cmd 0x%02x: %w", cmdType, err)
	}

	expectedType := cmdType + respOffset

	for attempt := 0; attempt < 10; attempt++ {
		resp, err := e.readOnePacket(tr, cmdType)
		if err != nil {
			return Packet{}, err
		}

		if resp.Type == expectedType {
			webutil.LogTrace("niimbot: cmd 0x%02x → resp 0x%02x [%s]", cmdType, resp.Type, webutil.HexDump(resp.Data, 16))
			return resp, nil
		}

		webutil.LogTrace("niimbot: cmd 0x%02x → skip unsolicited 0x%02x [%s]", cmdType, resp.Type, webutil.HexDump(resp.Data, 16))
	}

	return Packet{}, fmt.Errorf("cmd 0x%02x: expected resp 0x%02x, got too many unexpected packets", cmdType, expectedType)
}

// readOnePacket reads exactly one Niimbot packet from the transport.
// It synchronizes on the 0x55 0x55 header, skipping any stale bytes.
func (e *NiimbotEncoder) readOnePacket(tr transport.Transport, cmdType byte) (Packet, error) {
	// Sync to packet header 0x55 0x55
	var prev byte
	oneByte := make([]byte, 1)
	for i := 0; i < 1024; i++ {
		if err := readFull(tr, oneByte); err != nil {
			return Packet{}, fmt.Errorf("sync header for cmd 0x%02x: %w", cmdType, err)
		}
		if prev == 0x55 && oneByte[0] == 0x55 {
			break
		}
		prev = oneByte[0]
		if i == 1023 {
			return Packet{}, fmt.Errorf("cmd 0x%02x: could not sync to packet header", cmdType)
		}
	}

	// Read type + len
	typLen := make([]byte, 2)
	if err := readFull(tr, typLen); err != nil {
		return Packet{}, fmt.Errorf("read type/len for cmd 0x%02x: %w", cmdType, err)
	}

	dlen := int(typLen[1])
	// Read data + checksum + 0xAA 0xAA
	tail := make([]byte, dlen+3)
	if err := readFull(tr, tail); err != nil {
		return Packet{}, fmt.Errorf("read body for cmd 0x%02x: %w", cmdType, err)
	}

	full := make([]byte, 0, 4+dlen+3)
	full = append(full, 0x55, 0x55)
	full = append(full, typLen...)
	full = append(full, tail...)

	resp, err := ParsePacket(full)
	if err != nil {
		return Packet{}, fmt.Errorf("parse resp for cmd 0x%02x: %w", cmdType, err)
	}

	return resp, nil
}

// transceiveAnyResponse sends a packet and returns the first response packet regardless of type.
// Used for commands like Heartbeat where multiple response types are valid (0xDD, 0xDE, 0xD9).
func (e *NiimbotEncoder) transceiveAnyResponse(tr transport.Transport, cmdType byte, data []byte) (Packet, error) {
	pkt := Packet{Type: cmdType, Data: data}
	if _, err := tr.Write(pkt.ToBytes()); err != nil {
		return Packet{}, fmt.Errorf("write cmd 0x%02x: %w", cmdType, err)
	}
	return e.readOnePacket(tr, cmdType)
}

// sendOnly sends a fire-and-forget packet (no response expected).
func (e *NiimbotEncoder) sendOnly(tr transport.Transport, cmdType byte, data []byte) error {
	pkt := Packet{Type: cmdType, Data: data}
	_, err := tr.Write(pkt.ToBytes())
	return err
}

// encodeRow converts one image row to 1-bit packed bytes (MSB = leftmost pixel).
// Pixels are inverted: white (high Y) → 0 bit, black (low Y) → 1 bit.
func (e *NiimbotEncoder) encodeRow(img image.Image, y, xStart, width int) []byte {
	bytesPerRow := (width + 7) / 8
	result := make([]byte, bytesPerRow)

	for x := 0; x < width; x++ {
		c := img.At(xStart+x, y)
		gray := color.GrayModel.Convert(c).(color.Gray)
		// Invert: black pixel (low Y) → bit 1, white pixel (high Y) → bit 0
		if gray.Y < 128 {
			byteIdx := x / 8
			bitIdx := uint(7 - (x % 8)) // MSB = leftmost
			result[byteIdx] |= 1 << bitIdx
		}
	}

	return result
}

// readFull reads exactly len(buf) bytes from the transport.
func readFull(tr transport.Transport, buf []byte) error {
	total := 0
	for total < len(buf) {
		n, err := tr.Read(buf[total:])
		if err != nil {
			return err
		}
		total += n
		if n == 0 {
			return fmt.Errorf("transport returned 0 bytes, got %d/%d", total, len(buf))
		}
	}
	return nil
}

// isAllZero returns true if all bytes in the slice are zero.
func isAllZero(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}
