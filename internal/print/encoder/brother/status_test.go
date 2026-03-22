package brother

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	// Base: valid status with no errors, 62mm continuous media.
	base := [statusSize]byte{}
	base[0] = statusHeader0 // 0x80
	base[1] = statusHeader1 // 0x20
	base[2] = 'B'           // Brother
	base[3] = '0'           // series
	base[10] = 62           // media width mm
	base[11] = mediaContinuous
	base[17] = 0 // continuous = 0
	base[18] = statusTypeReply

	tests := []struct {
		name    string
		modify  func(d *[statusSize]byte)
		wantErr bool
		check   func(t *testing.T, st brotherStatus)
	}{
		{
			name:   "happy path — 62mm continuous, no errors",
			modify: func(d *[statusSize]byte) {},
			check: func(t *testing.T, st brotherStatus) {
				if st.MediaWidth != 62 {
					t.Errorf("media width = %d, want 62", st.MediaWidth)
				}
				if st.MediaType != mediaContinuous {
					t.Errorf("media type = 0x%02X, want 0x%02X", st.MediaType, mediaContinuous)
				}
				if st.CoverOpen || st.NoMedia || st.CutterJam || st.Overheating {
					t.Error("expected no error flags")
				}
			},
		},
		{
			name: "cover open",
			modify: func(d *[statusSize]byte) {
				d[9] = errCoverOpen
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.CoverOpen {
					t.Error("expected CoverOpen = true")
				}
			},
		},
		{
			name: "no media",
			modify: func(d *[statusSize]byte) {
				d[8] = errNoMedia
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.NoMedia {
					t.Error("expected NoMedia = true")
				}
			},
		},
		{
			name: "cutter jam",
			modify: func(d *[statusSize]byte) {
				d[8] = errCutterJam
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.CutterJam {
					t.Error("expected CutterJam = true")
				}
			},
		},
		{
			name: "end of media",
			modify: func(d *[statusSize]byte) {
				d[8] = errEndOfMedia
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.EndOfMedia {
					t.Error("expected EndOfMedia = true")
				}
			},
		},
		{
			name: "overheating",
			modify: func(d *[statusSize]byte) {
				d[9] = errOverheating
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.Overheating {
					t.Error("expected Overheating = true")
				}
			},
		},
		{
			name: "multiple errors",
			modify: func(d *[statusSize]byte) {
				d[8] = errNoMedia | errCutterJam
				d[9] = errCoverOpen
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.NoMedia {
					t.Error("expected NoMedia = true")
				}
				if !st.CutterJam {
					t.Error("expected CutterJam = true")
				}
				if !st.CoverOpen {
					t.Error("expected CoverOpen = true")
				}
			},
		},
		{
			name: "printing done status",
			modify: func(d *[statusSize]byte) {
				d[18] = statusTypePrintDone
			},
			check: func(t *testing.T, st brotherStatus) {
				if !st.PrintingDone {
					t.Error("expected PrintingDone = true")
				}
			},
		},
		{
			name: "die-cut media 29x90mm",
			modify: func(d *[statusSize]byte) {
				d[10] = 29
				d[11] = mediaDieCut
				d[17] = 90
			},
			check: func(t *testing.T, st brotherStatus) {
				if st.MediaWidth != 29 {
					t.Errorf("media width = %d, want 29", st.MediaWidth)
				}
				if st.MediaLength != 90 {
					t.Errorf("media length = %d, want 90", st.MediaLength)
				}
				if st.MediaType != mediaDieCut {
					t.Errorf("media type = 0x%02X, want 0x%02X", st.MediaType, mediaDieCut)
				}
			},
		},
		{
			name: "invalid header byte 0",
			modify: func(d *[statusSize]byte) {
				d[0] = 0x00
			},
			wantErr: true,
		},
		{
			name: "invalid header byte 1",
			modify: func(d *[statusSize]byte) {
				d[1] = 0x00
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := base // copy
			tt.modify(&data)

			st, err := parseStatus(data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, st)
			}
		})
	}
}

func TestStatusError(t *testing.T) {
	tests := []struct {
		name    string
		status  brotherStatus
		wantNil bool
	}{
		{
			name:    "no errors",
			status:  brotherStatus{},
			wantNil: true,
		},
		{
			name:   "cover open",
			status: brotherStatus{CoverOpen: true},
		},
		{
			name:   "multiple errors",
			status: brotherStatus{CoverOpen: true, NoMedia: true, CutterJam: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := statusError(tt.status)
			if tt.wantNil && err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}
			if !tt.wantNil && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestMediaTypeName(t *testing.T) {
	tests := []struct {
		input byte
		want  string
	}{
		{mediaContinuous, "continuous"},
		{mediaDieCut, "die-cut"},
		{mediaTypeNone, "none"},
		{0xFF, "unknown(0xFF)"},
	}

	for _, tt := range tests {
		got := mediaTypeName(tt.input)
		if got != tt.want {
			t.Errorf("mediaTypeName(0x%02X) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
