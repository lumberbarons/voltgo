package ble

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	ErrNotConnected     = errors.New("not connected to device")
	ErrNoDevice         = errors.New("no device found")
	ErrTimeout          = errors.New("operation timeout")
	ErrNoService        = errors.New("service not found")
	ErrNoCharacteristic = errors.New("characteristic not found")
	ErrFrameTooLarge    = errors.New("frame exceeds device buffer")
)

// maxFrameSize is the size of the device's write characteristic buffer.
const maxFrameSize = 200

const (
	// connectAttempts bounds how many times a link is opened before the
	// failure reaches the caller.
	//
	// The abort this works around is intermittent, and how often it strikes
	// drifts over time: measured against a ZT-25.6V100Ah on BlueZ 5.64, runs
	// of dials that nearly all abort alternate with stretches that mostly
	// succeed. A dial that fails is torn down about half a second after the
	// link comes up, having exchanged no ATT traffic at all, so retrying
	// costs little. Six attempts covered every observed run; the worst case
	// is a few seconds spent before the error reaches the caller.
	connectAttempts = 6

	// connectRetryDelay separates attempts, leaving the adapter time to tear
	// the aborted link down before the next one is opened.
	connectRetryDelay = 500 * time.Millisecond
)

// peerAbortReasons are the BlueZ disconnect reasons that mean a link formed
// and was then torn down, rather than never forming at all. Only these are
// worth retrying: a missing device or an unreachable address fails the same
// way every time, and retrying it only delays the error the caller needs.
//
// BlueZ reports the reason as text on the D-Bus error and tinygo passes it
// through unchanged, so matching on that text is the only handle available.
var peerAbortReasons = []string{
	"le-connection-abort-by-local",
	"br-connection-canceled",
}

// abortedByPeer reports whether err is a link that was established and then
// aborted, as opposed to one that never came up.
func abortedByPeer(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	for _, reason := range peerAbortReasons {
		if strings.Contains(msg, reason) {
			return true
		}
	}
	return false
}

// dialFunc opens the low-level link. It matches (*bluetooth.Adapter).Connect,
// so the real adapter satisfies it directly and tests can substitute a stub
// without an adapter or a battery.
type dialFunc func(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error)

// Connection represents a BLE connection to a battery
type Connection struct {
	adapter    *bluetooth.Adapter
	dial       dialFunc
	attempts   int
	retryDelay time.Duration
	device     bluetooth.Device
	service    bluetooth.DeviceService
	writeChar  bluetooth.DeviceCharacteristic
	notifyChar bluetooth.DeviceCharacteristic
	connected  bool
	mu         sync.RWMutex
	responses  chan []byte
	done       chan struct{}
}

// NewConnection creates a new BLE connection handler
func NewConnection() (*Connection, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("failed to enable BLE adapter: %w", err)
	}

	return &Connection{
		adapter:    adapter,
		dial:       adapter.Connect,
		attempts:   connectAttempts,
		retryDelay: connectRetryDelay,
		responses:  make(chan []byte, 10),
		done:       make(chan struct{}),
	}, nil
}

// Scan scans for nearby battery devices
func (c *Connection) Scan(ctx context.Context, duration time.Duration) ([]bluetooth.ScanResult, error) {
	var devices []bluetooth.ScanResult
	var mu sync.Mutex

	scanCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	err := c.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		mu.Lock()
		devices = append(devices, result)
		mu.Unlock()

		select {
		case <-scanCtx.Done():
			//nolint:errcheck // Best effort stop scan in callback
			adapter.StopScan()
		default:
		}
	})

	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	<-scanCtx.Done()
	//nolint:errcheck // Best effort stop scan
	c.adapter.StopScan()

	return devices, nil
}

// Connect connects to a BLE device by address and prepares the battery's
// write and notify characteristics.
//
// A link the battery accepts and then aborts is retried rather than returned
// on the first failure; see peerAbortReasons for why that failure is treated
// differently from every other connect error.
func (c *Connection) Connect(ctx context.Context, address bluetooth.Address) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return errors.New("already connected")
	}

	dial, attempts, delay := c.dial, c.attempts, c.retryDelay
	if dial == nil {
		dial = c.adapter.Connect
	}
	if attempts <= 0 {
		attempts = connectAttempts
	}
	if delay <= 0 {
		delay = connectRetryDelay
	}

	return retryPeerAborts(ctx, attempts, delay, func() error {
		return c.establish(dial, address)
	})
}

// retryPeerAborts runs establish until it succeeds, fails for a reason other
// than a peer abort, or exhausts the attempt budget. The backoff between
// attempts is skipped after the final one, and a cancelled context ends the
// retries with both the abort and the cancellation preserved.
func retryPeerAborts(ctx context.Context, attempts int, delay time.Duration, establish func() error) error {
	var err error

	for attempt := 1; attempt <= attempts; attempt++ {
		err = establish()
		if err == nil {
			return nil
		}
		if !abortedByPeer(err) || attempt == attempts {
			return err
		}
		if waitErr := sleepCtx(ctx, delay); waitErr != nil {
			return errors.Join(err, waitErr)
		}
	}

	return err
}

// sleepCtx waits for d, returning early if ctx is done first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// establish opens one link and prepares the battery's characteristics.
//
// Service discovery follows the dial immediately and with nothing in
// between. The battery drops a link whose ATT MTU negotiation is slow to
// arrive, so work inserted between these two steps eats into that budget;
// anything that can be done before the dial belongs before it, not here.
// On a link that survives, an hci trace puts the MTU request about half a
// second after the connection completes and the response about 300ms later,
// comfortably inside the window.
func (c *Connection) establish(dial dialFunc, address bluetooth.Address) error {
	device, err := dial(address, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(30 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.device = device

	services, err := device.DiscoverServices([]bluetooth.UUID{ServiceUUID})
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover services: %w", err)
	}
	if len(services) == 0 {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return ErrNoService
	}
	c.service = services[0]

	chars, err := c.service.DiscoverCharacteristics([]bluetooth.UUID{
		WriteCharacteristicUUID,
		NotifyCharacteristicUUID,
	})
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover characteristics: %w", err)
	}

	var haveWrite, haveNotify bool
	for _, char := range chars {
		switch char.UUID() {
		case WriteCharacteristicUUID:
			c.writeChar = char
			haveWrite = true
		case NotifyCharacteristicUUID:
			c.notifyChar = char
			haveNotify = true
		}
	}
	if !haveWrite || !haveNotify {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return ErrNoCharacteristic
	}

	// Fresh channels per connection. The notification callback closes over
	// them (rather than reading the struct fields) so a callback from a
	// previous connection can never write into the current one's channel.
	responses := make(chan []byte, 10)
	if err := c.notifyChar.EnableNotifications(func(buf []byte) {
		data := make([]byte, len(buf))
		copy(data, buf)

		select {
		case responses <- data:
		default:
			// Drop if channel is full
		}
	}); err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to enable notifications: %w", err)
	}

	// Give the device a moment to settle after CCCD write
	time.Sleep(200 * time.Millisecond)

	c.responses = responses
	c.done = make(chan struct{})
	c.connected = true
	return nil
}

// Disconnect disconnects from the device
func (c *Connection) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if err := c.device.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	c.connected = false
	// Wake any reader blocked in ReadFrame. The responses channel is never
	// closed: a stale notification callback may still hold a reference, and
	// sending on a closed channel would panic.
	close(c.done)

	return nil
}

// IsConnected returns whether the connection is active
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// WriteFrame writes a raw frame to the device's write characteristic.
func (c *Connection) WriteFrame(_ context.Context, frame []byte) error {
	if len(frame) > maxFrameSize {
		return fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, len(frame))
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return ErrNotConnected
	}

	if _, err := c.writeChar.WriteWithoutResponse(frame); err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}
	return nil
}

// ReadFrame waits for the next notification frame, up to timeout.
func (c *Connection) ReadFrame(ctx context.Context, timeout time.Duration) ([]byte, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, ErrNotConnected
	}
	responses, done := c.responses, c.done
	c.mu.RUnlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case data := <-responses:
		return data, nil
	case <-done:
		return nil, ErrNotConnected
	case <-timer.C:
		return nil, ErrTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Request writes a frame and waits for the response notification. Any stale
// notifications queued before the write are discarded.
func (c *Connection) Request(ctx context.Context, frame []byte, timeout time.Duration) ([]byte, error) {
	c.mu.RLock()
	responses := c.responses
	c.mu.RUnlock()

drain:
	for {
		select {
		case <-responses:
		default:
			break drain
		}
	}

	if err := c.WriteFrame(ctx, frame); err != nil {
		return nil, err
	}
	return c.ReadFrame(ctx, timeout)
}
