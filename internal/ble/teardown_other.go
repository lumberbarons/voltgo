//go:build !linux

package ble

import (
	"context"
	"time"

	"tinygo.org/x/bluetooth"
)

// waitForTeardown waits out the ceiling rather than watching for the
// disconnect, because the watch is a BlueZ one and BlueZ is Linux-only.
//
// Nothing is lost by that here: every abort worth retrying is reported by
// BlueZ (see peerAbortReasons), so this path is reached only when some other
// stack produces an error that happens to read like one, and then a plain
// wait is the right conservative answer.
func waitForTeardown(ctx context.Context, _ bluetooth.Address, ceiling time.Duration) error {
	return sleepCtx(ctx, ceiling)
}
