package ble

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConnection builds a Connection in the connected state without a
// real adapter. Only the channel plumbing is usable; methods that touch the
// GATT characteristics need hardware and are covered by the hardware suite.
func newTestConnection() *Connection {
	return &Connection{
		connected: true,
		responses: make(chan []byte, 10),
		done:      make(chan struct{}),
	}
}

func TestReadFrame_NotConnected(t *testing.T) {
	c := &Connection{}
	_, err := c.ReadFrame(context.Background(), time.Second)
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestReadFrame_DeliversQueuedFrame(t *testing.T) {
	c := newTestConnection()
	c.responses <- []byte{0x01, 0x02}

	frame, err := c.ReadFrame(context.Background(), time.Second)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02}, frame)
}

func TestReadFrame_Timeout(t *testing.T) {
	c := newTestConnection()

	start := time.Now()
	_, err := c.ReadFrame(context.Background(), 20*time.Millisecond)
	assert.ErrorIs(t, err, ErrTimeout)
	assert.Less(t, time.Since(start), time.Second)
}

func TestReadFrame_ContextCancelled(t *testing.T) {
	c := newTestConnection()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.ReadFrame(ctx, time.Second)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestReadFrame_UnblocksOnDisconnect is the regression test for a race where
// Disconnect closed and reassigned the responses channel while a reader was
// blocked on it: the reader received the closed channel's zero value and
// returned a nil frame with no error. A blocked ReadFrame must instead
// surface ErrNotConnected as soon as the connection is torn down.
func TestReadFrame_UnblocksOnDisconnect(t *testing.T) {
	c := newTestConnection()

	type result struct {
		frame []byte
		err   error
	}
	got := make(chan result, 1)
	go func() {
		frame, err := c.ReadFrame(context.Background(), 10*time.Second)
		got <- result{frame, err}
	}()

	// Give the reader time to block, then tear down as Disconnect does.
	time.Sleep(20 * time.Millisecond)
	c.mu.Lock()
	c.connected = false
	close(c.done)
	c.mu.Unlock()

	select {
	case r := <-got:
		assert.ErrorIs(t, r.err, ErrNotConnected)
		assert.Nil(t, r.frame)
	case <-time.After(2 * time.Second):
		t.Fatal("ReadFrame did not unblock on disconnect")
	}
}

func TestWriteFrame_TooLarge(t *testing.T) {
	c := newTestConnection()
	err := c.WriteFrame(context.Background(), make([]byte, maxFrameSize+1))
	assert.ErrorIs(t, err, ErrFrameTooLarge)
}

func TestWriteFrame_NotConnected(t *testing.T) {
	c := &Connection{}
	err := c.WriteFrame(context.Background(), []byte{0x01})
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestRequest_DrainsStaleResponses(t *testing.T) {
	// Stale notifications queued before a request must not be returned as
	// its response. Not connected, so the write fails after the drain —
	// which is exactly what lets us observe the drain without hardware.
	c := newTestConnection()
	c.connected = false
	c.responses <- []byte{0xde, 0xad}
	c.responses <- []byte{0xbe, 0xef}

	_, err := c.Request(context.Background(), []byte{0x01}, time.Second)
	assert.ErrorIs(t, err, ErrNotConnected)
	assert.Empty(t, c.responses)
}

func TestIsConnected(t *testing.T) {
	c := &Connection{}
	assert.False(t, c.IsConnected())
	c.connected = true
	assert.True(t, c.IsConnected())
}
