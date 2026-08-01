//go:build linux

package ble

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDevicePathSuffix_MatchesBlueZNaming ties the derived suffix to the
// object path BlueZ actually exports; a mismatch would leave the wait
// watching a device that never reports anything.
func TestDevicePathSuffix_MatchesBlueZNaming(t *testing.T) {
	address, err := ParseAddress(testAddress)
	require.NoError(t, err)

	suffix := devicePathSuffix(address)

	assert.Equal(t, "dev_A4_C1_37_43_A4_33", suffix)
	assert.True(t, strings.HasSuffix(string(testDevicePath), suffix),
		"the suffix must match the tail of the device's object path")
}

// TestDevicePathSuffix_NormalisesLowercaseInput covers an address parsed from
// user input: BlueZ names its objects in uppercase hex.
func TestDevicePathSuffix_NormalisesLowercaseInput(t *testing.T) {
	address, err := ParseAddress(strings.ToLower(testAddress))
	require.NoError(t, err)

	assert.Equal(t, "dev_A4_C1_37_43_A4_33", devicePathSuffix(address))
}

// TestWaitForTeardown_ReturnsCleanlyWhenTheresNothingToWatch exercises the
// real entry point on a host with no BlueZ or no such device, which is what
// CI and any container look like. Every way of failing to observe a teardown
// has to come back without an error and inside the ceiling: the retry dials
// again regardless, so a missing watch must not become a connect failure.
func TestWaitForTeardown_ReturnsCleanlyWhenTheresNothingToWatch(t *testing.T) {
	address, err := ParseAddress(testAddress)
	require.NoError(t, err)

	ceiling := 100 * time.Millisecond
	start := time.Now()

	require.NoError(t, waitForTeardown(context.Background(), address, ceiling),
		"an unobservable teardown is not a connect failure")
	assert.Less(t, time.Since(start), ceiling+2*time.Second, "the ceiling must bound the wait")
}
