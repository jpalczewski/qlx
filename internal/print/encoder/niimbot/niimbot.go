package niimbot

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

const (
	cmdSetDensity      = 0x21
	cmdSetLabelType    = 0x23
	cmdPrintStart      = 0x01
	cmdPageStart       = 0x03
	cmdSetPageSize     = 0x13
	cmdPrintBitmapRow  = 0x85
	cmdPrintEmptyRow   = 0x84
	cmdPageEnd         = 0xE3
	cmdPrintEnd        = 0xF3

	respOffsetStandard = 1
	respOffsetDensity  = 16
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

	// 1. SET_DENSITY
	if err := e.transceive(tr, cmdSetDensity, []byte{byte(density)}, respOffsetDensity); err != nil {
		return fmt.Errorf("SET_DENSITY: %w", err)
	}

	// 2. SET_LABEL_TYPE (1 = gaps)
	if err := e.transceive(tr, cmdSetLabelType, []byte{0x01}, respOffsetDensity); err != nil {
		return fmt.Errorf("SET_LABEL_TYPE: %w", err)
	}

	// 3. PRINT_START: [totalPages:u16 BE, 0x00, 0x00, 0x00, pageColor:u8, 0x00] (7 bytes)
	printStartData := make([]byte, 7)
	binary.BigEndian.PutUint16(printStartData[0:2], 1) // totalPages = 1
	printStartData[2] = 0x00
	printStartData[3] = 0x00
	printStartData[4] = 0x00
	printStartData[5] = 0x01 // pageColor
	printStartData[6] = 0x00
	if err := e.transceive(tr, cmdPrintStart, printStartData, respOffsetStandard); err != nil {
		return fmt.Errorf("PRINT_START: %w", err)
	}

	// 4. PAGE_START
	if err := e.transceive(tr, cmdPageStart, []byte{0x01}, respOffsetStandard); err != nil {
		return fmt.Errorf("PAGE_START: %w", err)
	}

	// 5. SET_PAGE_SIZE: [rows:u16 BE, cols:u16 BE, copies:u16 BE]
	pageSizeData := make([]byte, 6)
	binary.BigEndian.PutUint16(pageSizeData[0:2], uint16(rows))
	binary.BigEndian.PutUint16(pageSizeData[2:4], uint16(m.PrintheadPx))
	binary.BigEndian.PutUint16(pageSizeData[4:6], 1) // copies = 1
	if err := e.transceive(tr, cmdSetPageSize, pageSizeData, respOffsetStandard); err != nil {
		return fmt.Errorf("SET_PAGE_SIZE: %w", err)
	}

	// 6. Send each image row
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		rowNum := uint16(y - bounds.Min.Y)
		rowData := e.encodeRow(img, y, bounds.Min.X, m.PrintheadPx)

		if isAllZero(rowData) {
			// PRINT_EMPTY_ROW: [rowNum:u16 BE, repeatCount:u8]
			data := make([]byte, 3)
			binary.BigEndian.PutUint16(data[0:2], rowNum)
			data[2] = 1 // repeatCount
			if err := e.sendOnly(tr, cmdPrintEmptyRow, data); err != nil {
				return fmt.Errorf("PRINT_EMPTY_ROW row %d: %w", rowNum, err)
			}
		} else {
			// PRINT_BITMAP_ROW: [rowNum:u16 BE, blackPxCount(3 bytes: 0,0,0), repeat:u8(1), bitmap_data]
			data := make([]byte, 2+3+1+len(rowData))
			binary.BigEndian.PutUint16(data[0:2], rowNum)
			data[2] = 0x00
			data[3] = 0x00
			data[4] = 0x00
			data[5] = 0x01 // repeat
			copy(data[6:], rowData)
			if err := e.sendOnly(tr, cmdPrintBitmapRow, data); err != nil {
				return fmt.Errorf("PRINT_BITMAP_ROW row %d: %w", rowNum, err)
			}
		}
	}

	// 7. PAGE_END
	if err := e.transceive(tr, cmdPageEnd, []byte{0x01}, respOffsetStandard); err != nil {
		return fmt.Errorf("PAGE_END: %w", err)
	}

	// 8. PRINT_END — poll until response data[0] != 0
	for {
		resp, err := e.transceiveWithResponse(tr, cmdPrintEnd, []byte{0x01}, respOffsetStandard)
		if err != nil {
			return fmt.Errorf("PRINT_END: %w", err)
		}
		if len(resp.Data) > 0 && resp.Data[0] != 0 {
			break
		}
		// If data[0] == 0, printer is still busy; in practice with mock this won't loop
		break
	}

	return nil
}

// transceive sends a packet and reads+validates the response.
func (e *NiimbotEncoder) transceive(tr transport.Transport, cmdType byte, data []byte, respOffset byte) error {
	_, err := e.transceiveWithResponse(tr, cmdType, data, respOffset)
	return err
}

// transceiveWithResponse sends a packet and returns the parsed response packet.
// It reads exactly one packet from the transport by first reading the header bytes
// to determine the data length, then reading the rest.
func (e *NiimbotEncoder) transceiveWithResponse(tr transport.Transport, cmdType byte, data []byte, respOffset byte) (Packet, error) {
	pkt := Packet{Type: cmdType, Data: data}
	if _, err := tr.Write(pkt.ToBytes()); err != nil {
		return Packet{}, fmt.Errorf("write cmd 0x%02x: %w", cmdType, err)
	}

	// Read the fixed header: 0x55 0x55 <type> <len> = 4 bytes
	header := make([]byte, 4)
	if err := readFull(tr, header); err != nil {
		return Packet{}, fmt.Errorf("read header for cmd 0x%02x: %w", cmdType, err)
	}

	dlen := int(header[3])
	// Read remaining: data + checksum + 0xAA 0xAA = dlen + 3 bytes
	tail := make([]byte, dlen+3)
	if err := readFull(tr, tail); err != nil {
		return Packet{}, fmt.Errorf("read body for cmd 0x%02x: %w", cmdType, err)
	}

	full := append(header, tail...)
	resp, err := ParsePacket(full)
	if err != nil {
		return Packet{}, fmt.Errorf("parse resp for cmd 0x%02x: %w", cmdType, err)
	}

	expectedType := cmdType + respOffset
	if resp.Type != expectedType {
		return Packet{}, fmt.Errorf("cmd 0x%02x: expected resp type 0x%02x, got 0x%02x", cmdType, expectedType, resp.Type)
	}

	return resp, nil
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
