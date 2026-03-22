package brother

import (
	"errors"
	"fmt"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// Compile-time check: BrotherEncoder implements StatusQuerier.
var _ encoder.StatusQuerier = (*BrotherEncoder)(nil)

const (
	// statusSize is the fixed size of a Brother QL status response.
	statusSize = 32

	// Status header bytes.
	statusHeader0 = 0x80
	statusHeader1 = 0x20

	// Status type values (byte 18).
	statusTypeReply       = 0x00
	statusTypePrintDone   = 0x01
	statusTypeError       = 0x02
	statusTypeNotify      = 0x05
	statusTypePhaseChange = 0x06

	// Media type: no media loaded.
	mediaTypeNone byte = 0x00

	// Error info 1 bit masks (byte 8).
	errNoMedia    = 0x01
	errEndOfMedia = 0x02
	errCutterJam  = 0x04

	// Error info 2 bit masks (byte 9).
	errCoverOpen    = 0x01
	errOverheating  = 0x04
	errReplaceMedia = 0x10
)

// brotherStatus holds parsed fields from a 32-byte QL status response.
type brotherStatus struct {
	// Raw protocol fields.
	ErrorInfo1  byte // byte 8
	ErrorInfo2  byte // byte 9
	MediaWidth  int  // byte 10 (mm)
	MediaType   byte // byte 11
	MediaLength int  // byte 17 (mm, 0 = continuous)
	StatusType  byte // byte 18
	PhaseType   byte // byte 19

	// Derived booleans.
	CoverOpen    bool
	NoMedia      bool
	CutterJam    bool
	EndOfMedia   bool
	Overheating  bool
	ReplaceMedia bool
	PrintingDone bool
}

// Connect performs the initial handshake: invalidate → init → read status.
func (e *BrotherEncoder) Connect(tr transport.Transport) error {
	// 1. Send 200 null bytes to invalidate any pending state.
	if _, err := tr.Write(make([]byte, 200)); err != nil {
		return fmt.Errorf("brother connect: invalidate: %w", err)
	}

	// 2. ESC @ — initialize.
	if _, err := tr.Write([]byte{0x1B, 0x40}); err != nil {
		return fmt.Errorf("brother connect: init: %w", err)
	}

	// 3. Request and read status.
	st, err := requestStatus(tr)
	if err != nil {
		return fmt.Errorf("brother connect: %w", err)
	}

	webutil.LogInfo("brother: connected — media=%dmm type=0x%02X errors1=0x%02X errors2=0x%02X",
		st.MediaWidth, st.MediaType, st.ErrorInfo1, st.ErrorInfo2)

	return nil
}

// Heartbeat sends a status request and returns parsed printer state.
func (e *BrotherEncoder) Heartbeat(tr transport.Transport) (encoder.HeartbeatResult, error) {
	st, err := requestStatus(tr)
	if err != nil {
		return encoder.HeartbeatResult{}, fmt.Errorf("brother heartbeat: %w", err)
	}

	result := encoder.HeartbeatResult{
		Battery:     -1, // QL-700 has no battery
		LidClosed:   !st.CoverOpen,
		PaperLoaded: !st.NoMedia && !st.EndOfMedia,
	}

	return result, statusError(st)
}

// HeartbeatInterval returns the polling interval for Brother QL printers.
func (e *BrotherEncoder) HeartbeatInterval() time.Duration {
	return 1 * time.Second
}

// RfidInfo returns media information from the status response.
// QL-700 has no RFID, so we derive what we can from the status bytes.
func (e *BrotherEncoder) RfidInfo(tr transport.Transport) (encoder.RfidResult, error) {
	st, err := requestStatus(tr)
	if err != nil {
		return encoder.RfidResult{}, fmt.Errorf("brother rfid: %w", err)
	}

	return encoder.RfidResult{
		LabelType:     mediaTypeName(st.MediaType),
		TotalLabels:   -1, // unknown without RFID
		UsedLabels:    -1,
		LabelWidthMm:  st.MediaWidth,
		LabelHeightMm: st.MediaLength,
	}, nil
}

// requestStatus sends ESC i S and reads the 32-byte response.
func requestStatus(tr transport.Transport) (brotherStatus, error) {
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x53}); err != nil {
		return brotherStatus{}, fmt.Errorf("status request write: %w", err)
	}

	var buf [statusSize]byte
	n, err := readFull(tr, buf[:])
	if err != nil {
		return brotherStatus{}, fmt.Errorf("status read: %w", err)
	}
	if n < statusSize {
		return brotherStatus{}, fmt.Errorf("status read: got %d bytes, want %d", n, statusSize)
	}

	return parseStatus(buf)
}

// readFull reads exactly len(buf) bytes, retrying partial reads.
func readFull(tr transport.Transport, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := tr.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// parseStatus decodes a 32-byte Brother QL status response.
func parseStatus(data [statusSize]byte) (brotherStatus, error) {
	if data[0] != statusHeader0 || data[1] != statusHeader1 {
		return brotherStatus{}, fmt.Errorf("invalid status header: 0x%02X 0x%02X (want 0x80 0x20)",
			data[0], data[1])
	}

	st := brotherStatus{
		ErrorInfo1:  data[8],
		ErrorInfo2:  data[9],
		MediaWidth:  int(data[10]),
		MediaType:   data[11],
		MediaLength: int(data[17]),
		StatusType:  data[18],
		PhaseType:   data[19],
	}

	// Derive boolean flags.
	st.NoMedia = st.ErrorInfo1&errNoMedia != 0
	st.EndOfMedia = st.ErrorInfo1&errEndOfMedia != 0
	st.CutterJam = st.ErrorInfo1&errCutterJam != 0
	st.CoverOpen = st.ErrorInfo2&errCoverOpen != 0
	st.Overheating = st.ErrorInfo2&errOverheating != 0
	st.ReplaceMedia = st.ErrorInfo2&errReplaceMedia != 0
	st.PrintingDone = st.StatusType == statusTypePrintDone

	return st, nil
}

// statusError returns a combined error string if any error flags are set, or nil.
func statusError(st brotherStatus) error {
	var errs []string
	if st.CoverOpen {
		errs = append(errs, "cover open")
	}
	if st.NoMedia {
		errs = append(errs, "no media")
	}
	if st.EndOfMedia {
		errs = append(errs, "end of media")
	}
	if st.CutterJam {
		errs = append(errs, "cutter jam")
	}
	if st.Overheating {
		errs = append(errs, "overheating")
	}
	if st.ReplaceMedia {
		errs = append(errs, "replace media")
	}

	if len(errs) == 0 {
		return nil
	}

	msg := "brother: "
	for i, e := range errs {
		if i > 0 {
			msg += ", "
		}
		msg += e
	}
	return errors.New(msg)
}

// mediaTypeName returns a human-readable media type string.
func mediaTypeName(mt byte) string {
	switch mt {
	case mediaContinuous:
		return "continuous"
	case mediaDieCut:
		return "die-cut"
	case mediaTypeNone:
		return "none"
	default:
		return fmt.Sprintf("unknown(0x%02X)", mt)
	}
}
