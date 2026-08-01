package ble

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tinygo.org/x/bluetooth"
)

// errBlueZAbort is the error BlueZ surfaces when a link is established and then
// dropped before the ATT MTU exchange completes. It reaches this package
// wrapped by tinygo's adapter, so the tests wrap it the same way.
var errBlueZAbort = errors.New("bluetooth: failed to connect: le-connection-abort-by-local")

func TestAbortedByPeer_RecognisesBlueZAbort(t *testing.T) {
	assert.True(t, abortedByPeer(errBlueZAbort))
}

func TestAbortedByPeer_RecognisesWrappedAbort(t *testing.T) {
	wrapped := fmt.Errorf("failed to connect: %w", errBlueZAbort)
	assert.True(t, abortedByPeer(wrapped))
}

func TestAbortedByPeer_IgnoresUnrelatedErrors(t *testing.T) {
	assert.False(t, abortedByPeer(nil))
	assert.False(t, abortedByPeer(ErrNoDevice))
	assert.False(t, abortedByPeer(errors.New("bluetooth: no adapter")))
}

func TestRetryPeerAborts_SucceedsWithoutRetryWhenFirstAttemptWorks(t *testing.T) {
	attempts := 0
	err := retryPeerAborts(context.Background(), 3, time.Millisecond, func() error {
		attempts++
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, attempts, "a successful dial must not be repeated")
}

// TestRetryPeerAborts_RetriesAbortThenSucceeds is the core of the fix: the
// abort is timing-sensitive, so the same call that failed once can succeed
// moments later and must not be surfaced on the first failure.
func TestRetryPeerAborts_RetriesAbortThenSucceeds(t *testing.T) {
	attempts := 0
	err := retryPeerAborts(context.Background(), 3, time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return errBlueZAbort
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryPeerAborts_StopsAtAttemptBudget(t *testing.T) {
	attempts := 0
	err := retryPeerAborts(context.Background(), 3, time.Millisecond, func() error {
		attempts++
		return errBlueZAbort
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, errBlueZAbort, "the last abort must reach the caller")
	assert.Equal(t, 3, attempts, "the attempt budget bounds the retries")
}

// TestRetryPeerAborts_DoesNotRetryUnrelatedErrors keeps the retry narrow: a
// missing device or a bad address will fail identically every time, and
// retrying it only delays the error the caller needs to see.
func TestRetryPeerAborts_DoesNotRetryUnrelatedErrors(t *testing.T) {
	attempts := 0
	err := retryPeerAborts(context.Background(), 3, time.Millisecond, func() error {
		attempts++
		return ErrNoDevice
	})

	assert.ErrorIs(t, err, ErrNoDevice)
	assert.Equal(t, 1, attempts, "only peer aborts are retried")
}

func TestRetryPeerAborts_CancelledContextStopsRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	err := retryPeerAborts(ctx, 5, time.Hour, func() error {
		attempts++
		cancel()
		return errBlueZAbort
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, err, errBlueZAbort, "the abort that prompted the retry is preserved")
	assert.Equal(t, 1, attempts, "a cancelled context must not start another attempt")
}

// TestRetryPeerAborts_NoDelayAfterFinalAttempt guards against waiting out the
// backoff for a retry that will never happen.
func TestRetryPeerAborts_NoDelayAfterFinalAttempt(t *testing.T) {
	start := time.Now()
	err := retryPeerAborts(context.Background(), 2, 300*time.Millisecond, func() error {
		return errBlueZAbort
	})

	require.Error(t, err)
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 300*time.Millisecond, "one backoff between the two attempts")
	assert.Less(t, elapsed, 600*time.Millisecond, "no backoff after the final attempt")
}

// TestConnect_RetriesDialAbort checks the wiring rather than the loop: a dial
// that aborts must be redialled through Connection.Connect itself.
func TestConnect_RetriesDialAbort(t *testing.T) {
	dials := 0
	c := &Connection{
		attempts:   3,
		retryDelay: time.Millisecond,
		dial: func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
			dials++
			return bluetooth.Device{}, errBlueZAbort
		},
	}

	err := c.Connect(context.Background(), bluetooth.Address{})

	require.Error(t, err)
	assert.ErrorIs(t, err, errBlueZAbort)
	assert.Equal(t, 3, dials, "Connect must redial an aborted link")
	assert.False(t, c.IsConnected())
}

func TestConnect_DoesNotRetryUnrelatedDialFailure(t *testing.T) {
	dials := 0
	c := &Connection{
		attempts:   3,
		retryDelay: time.Millisecond,
		dial: func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
			dials++
			return bluetooth.Device{}, ErrNoDevice
		},
	}

	err := c.Connect(context.Background(), bluetooth.Address{})

	require.Error(t, err)
	assert.Equal(t, 1, dials)
}

// TestConnect_RejectsSecondConnect preserves the existing guard: the retry
// loop must not run at all when a link is already up.
func TestConnect_RejectsSecondConnect(t *testing.T) {
	dials := 0
	c := &Connection{
		connected: true,
		dial: func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
			dials++
			return bluetooth.Device{}, nil
		},
	}

	err := c.Connect(context.Background(), bluetooth.Address{})

	require.Error(t, err)
	assert.Equal(t, 0, dials)
}

// TestConnect_AppliesDefaultsToZeroValueConnection covers a Connection built
// without NewConnection: it must still retry rather than make a single
// attempt with a zero budget.
func TestConnect_AppliesDefaultsToZeroValueConnection(t *testing.T) {
	dials := 0
	c := &Connection{
		retryDelay: time.Millisecond,
		dial: func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
			dials++
			return bluetooth.Device{}, errBlueZAbort
		},
	}

	err := c.Connect(context.Background(), bluetooth.Address{})

	require.Error(t, err)
	assert.Equal(t, connectAttempts, dials)
}
