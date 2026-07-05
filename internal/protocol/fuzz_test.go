package protocol

import (
	"encoding/hex"
	"testing"
)

// The parsers consume bytes straight off the radio, so they must never
// panic on malformed input. Seeds are live captures plus known error shapes;
// run longer campaigns with e.g. `go test -fuzz=FuzzParseReadResponse ./protocol`.

func FuzzParseReadResponse(f *testing.F) {
	seeds := []string{
		"0103020a7bfec7", // live single-register response
		"0103520a7b00000d160d180d190d1c0d1f0d1c0d1b0d1b" +
			"00000000000000000000000000000000001b001b001b00630064006400630000" +
			"0000000000000002000015752a001b1b00000000000803e8000000000000bd78", // live status block
		"018302a1c0", // exception response shape
		"01",         // short
		"",
	}
	for _, s := range seeds {
		b, err := hex.DecodeString(s)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, frame []byte) {
		regs, err := ParseReadResponse(frame, DefaultSlaveAddr)
		if err != nil {
			return
		}
		// Anything accepted must have had a valid CRC and a payload whose
		// byte count matches the register slice.
		if !VerifyCRC(frame) {
			t.Fatalf("accepted frame with bad CRC: %x", frame)
		}
		if len(regs)*2 != int(frame[2]) {
			t.Fatalf("register count %d inconsistent with byte count %d", len(regs), frame[2])
		}
	})
}

func FuzzParseBMSInfo(f *testing.F) {
	// Seed with the live status block encoded as big-endian bytes.
	live := make([]byte, 0, StatusRegisterCount*2)
	for _, r := range []uint16{2683, 0, 3350, 3352, 3353, 3356, 3359, 3356, 3355, 3355,
		0, 0, 0, 0, 0, 0, 0, 0, 27, 27, 27, 99, 100, 100, 99,
		0, 0, 0, 0, 2, 0, 0x1575, 0x2a00, 0x1b1b, 0, 0, 8, 1000, 0, 0, 0} {
		live = append(live, byte(r>>8), byte(r))
	}
	f.Add(live)
	f.Add([]byte{})
	f.Add(make([]byte, 10))

	f.Fuzz(func(t *testing.T, data []byte) {
		regs := make([]uint16, len(data)/2)
		for i := range regs {
			regs[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
		}

		info, err := ParseBMSInfo(regs)
		if err != nil {
			return
		}
		if info.CellCount < 1 || info.CellCount > 16 {
			t.Fatalf("accepted implausible cell count %d", info.CellCount)
		}
		if len(info.CellVoltages) != info.CellCount {
			t.Fatalf("cell voltage slice length %d != cell count %d", len(info.CellVoltages), info.CellCount)
		}
	})
}

func FuzzParseDeviceInfo(f *testing.F) {
	f.Add([]byte{0x54, 0x43, 0x00, 0x00})
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0x00, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		regs := make([]uint16, len(data)/2)
		for i := range regs {
			regs[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
		}

		info := ParseDeviceInfo(regs)
		for _, s := range info.Strings {
			if len(s) < 2 {
				t.Fatalf("kept sub-minimum-length field %q", s)
			}
			for _, r := range s {
				if r < 0x20 || r > 0x7E {
					t.Fatalf("kept non-printable field %q", s)
				}
			}
		}
	})
}
