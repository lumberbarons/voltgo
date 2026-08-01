//go:build hardware

package ble

import (
	"context"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/lumberbarons/voltgo/internal/protocol"
)

// This file is the instrument behind the connectAttempts figure in
// connection.go. It needs a battery, so it is kept behind a build tag and out
// of `make test`:
//
//	GOOS=linux GOARCH=arm64 go test -c -tags hardware ./internal/ble -o ble.test
//	VOLTGO_ADDR=A4:C1:37:43:A4:33 ./ble.test -test.run TestHardware_DialCounts -test.v
//
// Settings, all optional but VOLTGO_ADDR:
//
//	VOLTGO_ADDR    the battery's MAC
//	VOLTGO_RUNS    how many connects to make (default 20)
//	VOLTGO_MODE    what separates the retries: "teardown" (default) or "fixed"
//	VOLTGO_BUDGET  attempts per connect (default 20)
//
// The budget defaults well above the shipped connectAttempts on purpose: the
// question being measured is how many dials a connect actually needs, and a
// budget set to the current answer could only ever confirm itself.

const fixedRetryDelay = 500 * time.Millisecond

func TestHardware_DialCounts(t *testing.T) {
	address := requireAddress(t)
	runs := envInt(t, "VOLTGO_RUNS", 20)
	budget := envInt(t, "VOLTGO_BUDGET", 20)
	mode := envString("VOLTGO_MODE", "teardown")

	conn, err := NewConnection()
	if err != nil {
		t.Fatalf("open adapter: %v", err)
	}
	conn.attempts = budget

	dialer := conn.dial
	dials := 0
	conn.dial = func(a bluetooth.Address, p bluetooth.ConnectionParams) (bluetooth.Device, error) {
		dials++
		return dialer(a, p)
	}

	switch mode {
	case "teardown":
		// The shipped behaviour: wait for BlueZ to report the link gone.
	case "fixed":
		// What shipped before: a flat delay standing in for the teardown.
		conn.awaitTeardown = func(ctx context.Context, _ bluetooth.Address, _ time.Duration) error {
			return sleepCtx(ctx, fixedRetryDelay)
		}
	default:
		t.Fatalf("VOLTGO_MODE must be teardown or fixed, got %q", mode)
	}

	t.Logf("mode=%s runs=%d budget=%d address=%s", mode, runs, budget, os.Getenv("VOLTGO_ADDR"))

	var (
		counts    []int
		durations []time.Duration
		failures  int
	)

	for run := 1; run <= runs; run++ {
		dials = 0
		start := time.Now()

		err := conn.Connect(context.Background(), address)
		elapsed := time.Since(start)

		switch {
		case err != nil:
			failures++
			t.Logf("run=%2d dials=%2d elapsed=%-8s FAILED: %v", run, dials, elapsed.Round(time.Millisecond), err)
		default:
			// A link that cannot answer a read is not a successful connect,
			// so every run is confirmed with real traffic before it counts.
			if err := readStatus(conn); err != nil {
				failures++
				t.Logf("run=%2d dials=%2d elapsed=%-8s CONNECTED BUT UNREADABLE: %v",
					run, dials, elapsed.Round(time.Millisecond), err)
			} else {
				counts = append(counts, dials)
				durations = append(durations, elapsed)
				t.Logf("run=%2d dials=%2d elapsed=%-8s ok", run, dials, elapsed.Round(time.Millisecond))
			}

			if err := conn.Disconnect(); err != nil {
				t.Logf("run=%2d disconnect: %v", run, err)
			}
		}

		// Let the adapter settle so one run does not bias the next.
		time.Sleep(2 * time.Second)
	}

	report(t, mode, counts, durations, failures, runs)
}

// readStatus performs the same register read the CLI does, as proof the link
// is usable rather than merely open.
func readStatus(conn *Connection) error {
	frame := protocol.NewReadRequest(protocol.DefaultSlaveAddr, 0, protocol.StatusRegisterCount)

	resp, err := conn.Request(context.Background(), frame, 5*time.Second)
	if err != nil {
		return err
	}
	if _, err := protocol.ParseReadResponse(resp, protocol.DefaultSlaveAddr); err != nil {
		return err
	}
	return nil
}

func report(t *testing.T, mode string, counts []int, durations []time.Duration, failures, runs int) {
	t.Helper()

	t.Logf("--- %s: %d/%d connects succeeded ---", mode, len(counts), runs)
	if len(counts) == 0 {
		t.Fatalf("no successful connects in %d runs", runs)
	}

	histogram := map[int]int{}
	worst, total := 0, 0
	for _, n := range counts {
		histogram[n]++
		total += n
		if n > worst {
			worst = n
		}
	}

	keys := make([]int, 0, len(histogram))
	for n := range histogram {
		keys = append(keys, n)
	}
	sort.Ints(keys)
	for _, n := range keys {
		t.Logf("dials=%2d  %d run(s)", n, histogram[n])
	}

	var elapsed time.Duration
	slowest := time.Duration(0)
	for _, d := range durations {
		elapsed += d
		if d > slowest {
			slowest = d
		}
	}

	t.Logf("dials: worst=%d mean=%.2f", worst, float64(total)/float64(len(counts)))
	t.Logf("connect time: worst=%s mean=%s",
		slowest.Round(time.Millisecond),
		(elapsed / time.Duration(len(durations))).Round(time.Millisecond))
	t.Logf("failed connects: %d", failures)
}

func requireAddress(t *testing.T) bluetooth.Address {
	t.Helper()

	raw := os.Getenv("VOLTGO_ADDR")
	if raw == "" {
		t.Skip("VOLTGO_ADDR not set; this test needs a battery")
	}

	address, err := ParseAddress(raw)
	if err != nil {
		t.Fatalf("parse VOLTGO_ADDR %q: %v", raw, err)
	}
	return address
}

func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()

	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse %s %q: %v", name, raw, err)
	}
	return n
}

func envString(name, fallback string) string {
	if raw := os.Getenv(name); raw != "" {
		return raw
	}
	return fallback
}
