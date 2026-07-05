package protocol

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCorpus validates the parsers against every captured frame in
// testdata/corpus. Each subdirectory is one battery model; adding a capture
// file there extends compatibility coverage without touching test code.
// See testdata/corpus/README.md for the file format.
func TestCorpus(t *testing.T) {
	models, err := os.ReadDir(filepath.Join("testdata", "corpus"))
	require.NoError(t, err)

	tested := 0
	for _, model := range models {
		if !model.IsDir() {
			continue
		}
		frames, err := filepath.Glob(filepath.Join("testdata", "corpus", model.Name(), "*.hex"))
		require.NoError(t, err)

		for _, path := range frames {
			tested++
			t.Run(model.Name()+"/"+filepath.Base(path), func(t *testing.T) {
				frame := readHexFrame(t, path)

				require.True(t, VerifyCRC(frame), "corpus frame must carry a valid CRC")
				regs, err := ParseReadResponse(frame, DefaultSlaveAddr)
				require.NoError(t, err)
				require.NotEmpty(t, regs)

				if strings.HasPrefix(filepath.Base(path), "status") {
					assertPlausibleStatus(t, regs)
				}
			})
		}
	}
	require.NotZero(t, tested, "corpus is empty — testdata/corpus not found?")
}

// assertPlausibleStatus checks model-independent invariants of a status
// block. Exact values belong in per-model tests; these bounds must hold for
// any compatible battery.
func assertPlausibleStatus(t *testing.T, regs []uint16) {
	t.Helper()

	require.Len(t, regs, StatusRegisterCount)

	info, err := ParseBMSInfo(regs)
	require.NoError(t, err)

	assert.Greater(t, info.Voltage, 8.0, "pack voltage")
	assert.Less(t, info.Voltage, 60.0, "pack voltage")
	assert.GreaterOrEqual(t, info.SOC, 0)
	assert.LessOrEqual(t, info.SOC, 100)
	assert.GreaterOrEqual(t, info.SOH, 0)
	assert.LessOrEqual(t, info.SOH, 100)
	assert.GreaterOrEqual(t, info.CellCount, 4)
	assert.LessOrEqual(t, info.CellCount, 16)
	assert.Greater(t, info.FullCapacityAh, 0.0)

	for i, v := range info.CellVoltages {
		assert.Greater(t, v, 2.0, "cell %d voltage", i)
		assert.Less(t, v, 4.0, "cell %d voltage", i)
	}
	for i, temp := range info.Temperatures {
		assert.Greater(t, temp, -40, "temp sensor %d", i)
		assert.Less(t, temp, 100, "temp sensor %d", i)
	}
}

// readHexFrame parses a corpus file: hex bytes with whitespace ignored and
// '#' line comments.
func readHexFrame(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var sb strings.Builder
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sb.WriteString(line)
	}

	frame, err := hex.DecodeString(sb.String())
	require.NoError(t, err, "invalid hex in %s", path)
	require.NotEmpty(t, frame, "no frame bytes in %s", path)
	return frame
}
