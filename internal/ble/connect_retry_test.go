package ble

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tinygo.org/x/bluetooth"
)

// errBlueZAbort is the error BlueZ surfaces when a link is established and then
// dropped before the ATT MTU exchange completes. It reaches this package
// wrapped by tinygo's adapter, so the tests wrap it the same way.
var errBlueZAbort = errors.New("bluetooth: failed to connect: le-connection-abort-by-local")

// noWait stands in for a teardown that has already finished by the time the
// retry looks, which is the common case once BlueZ has announced it.
func noWait(context.Context) error { return nil }

// waitFor is a wait that never observes a disconnect and so always runs out
// its ceiling — the worst case the ceiling exists to bound.
func waitFor(d time.Duration) func(context.Context) error {
	return func(ctx context.Context) error { return sleepCtx(ctx, d) }
}

// instantTeardown reports the aborted link gone straight away, keeping tests
// that are about the retry itself off the clock.
func instantTeardown(context.Context, bluetooth.Address, time.Duration) error { return nil }

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
	err := retryPeerAborts(context.Background(), 3, noWait, func() error {
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
	err := retryPeerAborts(context.Background(), 3, noWait, func() error {
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
	err := retryPeerAborts(context.Background(), 3, noWait, func() error {
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
	err := retryPeerAborts(context.Background(), 3, noWait, func() error {
		attempts++
		return ErrNoDevice
	})

	assert.ErrorIs(t, err, ErrNoDevice)
	assert.Equal(t, 1, attempts, "only peer aborts are retried")
}

func TestRetryPeerAborts_CancelledContextStopsRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	err := retryPeerAborts(ctx, 5, waitFor(time.Hour), func() error {
		attempts++
		cancel()
		return errBlueZAbort
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, err, errBlueZAbort, "the abort that prompted the retry is preserved")
	assert.Equal(t, 1, attempts, "a cancelled context must not start another attempt")
}

// TestRetryPeerAborts_WaitsForTeardownBeforeEachRedial pins the sequencing the
// retry depends on: the abort leaves a link BlueZ is still tearing down, so
// every redial has to follow a wait rather than race it.
func TestRetryPeerAborts_WaitsForTeardownBeforeEachRedial(t *testing.T) {
	var order []string
	wait := func(context.Context) error {
		order = append(order, "wait")
		return nil
	}

	err := retryPeerAborts(context.Background(), 3, wait, func() error {
		order = append(order, "dial")
		return errBlueZAbort
	})

	require.Error(t, err)
	assert.Equal(t, []string{"dial", "wait", "dial", "wait", "dial"}, order,
		"each redial must be preceded by a wait for the previous link to go")
}

// TestRetryPeerAborts_NoTeardownWaitAfterFinalAttempt guards against waiting
// for a teardown that no dial will follow.
func TestRetryPeerAborts_NoTeardownWaitAfterFinalAttempt(t *testing.T) {
	waits := 0
	wait := func(context.Context) error {
		waits++
		return nil
	}

	err := retryPeerAborts(context.Background(), 2, wait, func() error {
		return errBlueZAbort
	})

	require.Error(t, err)
	assert.Equal(t, 1, waits, "two attempts leave exactly one gap to wait in")
}

// TestRetryPeerAborts_RedialsAsSoonAsTeardownIsObserved is what the fixed
// delay could not do: with the real wait driven by an announced disconnect,
// the retries follow the signal instead of sitting out a ceiling that is
// orders of magnitude longer.
func TestRetryPeerAborts_RedialsAsSoonAsTeardownIsObserved(t *testing.T) {
	signals := make(chan *dbus.Signal, 4)
	wait := func(ctx context.Context) error {
		// BlueZ announces the teardown of the link that just aborted.
		signals <- disconnected(testDevicePath)
		return awaitDisconnect(ctx, signals, testDevicePath, time.Hour)
	}

	dials := 0
	start := time.Now()
	err := retryPeerAborts(context.Background(), 3, wait, func() error {
		dials++
		return errBlueZAbort
	})

	require.Error(t, err)
	assert.Equal(t, 3, dials)
	assert.Less(t, time.Since(start), time.Second,
		"an observed teardown must release the retry rather than wait out the ceiling")
}

// TestConnect_WaitsForTeardownOfTheAddressItDialled checks the wiring: the
// wait has to be told which device to watch, or it would watch the wrong one.
func TestConnect_WaitsForTeardownOfTheAddressItDialled(t *testing.T) {
	address, err := ParseAddress(testAddress)
	require.NoError(t, err)

	var watched []bluetooth.Address
	ceilings := []time.Duration{}
	c := &Connection{
		attempts: 3,
		awaitTeardown: func(_ context.Context, a bluetooth.Address, ceiling time.Duration) error {
			watched = append(watched, a)
			ceilings = append(ceilings, ceiling)
			return nil
		},
		dial: func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
			return bluetooth.Device{}, errBlueZAbort
		},
	}

	require.Error(t, c.Connect(context.Background(), address))
	assert.Equal(t, []bluetooth.Address{address, address}, watched,
		"the wait must watch the device that was dialled")
	assert.Equal(t, []time.Duration{teardownCeiling, teardownCeiling}, ceilings,
		"a Connection built without a ceiling must still be given the default")
}

// TestConnect_RetriesDialAbort checks the wiring rather than the loop: a dial
// that aborts must be redialled through Connection.Connect itself.
func TestConnect_RetriesDialAbort(t *testing.T) {
	dials := 0
	c := &Connection{
		attempts:      3,
		awaitTeardown: instantTeardown,
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
		attempts:      3,
		awaitTeardown: instantTeardown,
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
		awaitTeardown: instantTeardown,
		dial: func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
			dials++
			return bluetooth.Device{}, errBlueZAbort
		},
	}

	err := c.Connect(context.Background(), bluetooth.Address{})

	require.Error(t, err)
	assert.Equal(t, connectAttempts, dials)
}
