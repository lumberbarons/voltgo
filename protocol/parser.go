package protocol

import (
	"encoding/binary"
	"fmt"
)

// BMSInfo contains all battery management system data
type BMSInfo struct {
	Voltage          float32   // Pack voltage in Volts
	Current          float32   // Pack current in Amps (positive=charge, negative=discharge)
	CellVoltages     []float32 // Individual cell voltages in Volts
	CellCount        int       // Number of cells
	SOC              int       // State of Charge (0-100%)
	SOH              int       // State of Health (0-100%)
	CellTemperatures []int8    // 4 cell temperatures in Celsius
	StatusFlags      uint16    // Status bitmap
	ProtectionFlags  uint16    // Protection status bitmap
	WarningFlags     uint16    // Warning status bitmap
	HeatingActive    bool      // Is heating currently active
	HeatingSwitchOn  bool      // Is heating switch enabled
}

const (
	// Packet 0 offsets
	offsetVoltage       = 0x00
	offsetCurrent       = 0x02
	offsetCellVoltages  = 0x04
	offsetSOC           = 0x25
	offsetSOH           = 0x28
	offsetStatusFlags   = 0x2E
	offsetProtectFlags  = 0x30
	offsetHeatingStatus = 0x32
	offsetWarningFlags  = 0x36
	offsetCellTemps     = 0x42
	offsetCellCount     = 0x48
	offsetHeatingSwitch = 0x50

	// Constants
	minPacket0Length   = 0x49 // 73 bytes minimum
	fullPacket0Length  = 0x52 // 82 bytes for all fields
	maxCellCount       = 500
	defaultCellCount   = 16
	cellsPerPacket     = 16
	heatingActiveValue = 0x80
	heatingSwitchBit   = 0x10
)

// ParseBMSInfoResponse parses a BMS info response (command 0x03/0x04)
// Supports multi-packet responses for batteries with >16 cells
func ParseBMSInfoResponse(packets [][]byte) (*BMSInfo, error) {
	if len(packets) == 0 {
		return nil, fmt.Errorf("no packets provided")
	}

	packet0 := packets[0]
	if len(packet0) < minPacket0Length {
		return nil, fmt.Errorf("packet 0 too short: %d bytes, need at least %d", len(packet0), minPacket0Length)
	}

	info := &BMSInfo{}

	// Parse total voltage (offset 0x00, little-endian uint16, divide by 100)
	rawVoltage := binary.LittleEndian.Uint16(packet0[offsetVoltage : offsetVoltage+2])
	info.Voltage = float32(rawVoltage) / 100.0

	// Parse current (offset 0x02, little-endian uint16, divide by 10, handle sign)
	rawCurrent := binary.LittleEndian.Uint16(packet0[offsetCurrent : offsetCurrent+2])
	currentFloat := float32(rawCurrent) / 10.0
	// Handle negative current (discharging): if > 3276.8, subtract 6553.6
	if currentFloat > 3276.8 {
		currentFloat = currentFloat - 6553.6
	}
	info.Current = currentFloat

	// Parse cell count (offset 0x48, little-endian uint16)
	rawCellCount := binary.LittleEndian.Uint16(packet0[offsetCellCount : offsetCellCount+2])
	if rawCellCount > maxCellCount {
		rawCellCount = defaultCellCount
	}
	info.CellCount = int(rawCellCount)

	// Initialize cell voltages array
	info.CellVoltages = make([]float32, info.CellCount)

	// Parse first batch of cell voltages (up to 16 cells in packet 0)
	firstBatchCount := min(info.CellCount, cellsPerPacket)
	for i := 0; i < firstBatchCount; i++ {
		offset := offsetCellVoltages + (i * 2)
		rawCellVoltage := binary.LittleEndian.Uint16(packet0[offset : offset+2])
		info.CellVoltages[i] = float32(rawCellVoltage) / 1000.0
	}

	// Parse continuation packets for cells 16+
	for packetIdx := 1; packetIdx < len(packets); packetIdx++ {
		packetData := packets[packetIdx]
		startCell := packetIdx * cellsPerPacket
		if startCell >= info.CellCount {
			break // No more cells to parse
		}

		cellsInThisPacket := min(info.CellCount-startCell, cellsPerPacket)
		for i := 0; i < cellsInThisPacket; i++ {
			offset := i * 2
			if offset+2 > len(packetData) {
				return nil, fmt.Errorf("packet %d too short for cell data", packetIdx)
			}
			rawCellVoltage := binary.LittleEndian.Uint16(packetData[offset : offset+2])
			info.CellVoltages[startCell+i] = float32(rawCellVoltage) / 1000.0
		}
	}

	// Parse SOC (offset 0x25, single byte)
	info.SOC = int(packet0[offsetSOC])

	// Parse SOH (offset 0x28, little-endian uint16)
	info.SOH = int(binary.LittleEndian.Uint16(packet0[offsetSOH : offsetSOH+2]))

	// Parse status flags (offset 0x2E, little-endian uint16)
	info.StatusFlags = binary.LittleEndian.Uint16(packet0[offsetStatusFlags : offsetStatusFlags+2])

	// Parse protection flags (offset 0x30, little-endian uint16)
	info.ProtectionFlags = binary.LittleEndian.Uint16(packet0[offsetProtectFlags : offsetProtectFlags+2])

	// Parse heating status (offset 0x32, single byte, 0x80 = active)
	info.HeatingActive = (packet0[offsetHeatingStatus] == heatingActiveValue)

	// Parse warning flags (offset 0x36, little-endian uint16)
	info.WarningFlags = binary.LittleEndian.Uint16(packet0[offsetWarningFlags : offsetWarningFlags+2])

	// Parse cell temperatures (offset 0x42, 4 signed bytes)
	info.CellTemperatures = make([]int8, 4)
	for i := 0; i < 4; i++ {
		info.CellTemperatures[i] = int8(packet0[offsetCellTemps+i])
	}

	// Parse heating switch (offset 0x50, only if packet is long enough)
	if len(packet0) >= fullPacket0Length {
		heatingSwitch := binary.LittleEndian.Uint16(packet0[offsetHeatingSwitch : offsetHeatingSwitch+2])
		info.HeatingSwitchOn = (heatingSwitch & heatingSwitchBit) != 0
	}

	return info, nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ProtectionStatus represents decoded protection flags
type ProtectionStatus struct {
	OverVoltage          bool
	UnderVoltage         bool
	OverCurrent          bool
	OverTemperature      bool
	UnderTemperature     bool
	ShortCircuit         bool
	DischargeOverCurrent bool
	ChargeOverCurrent    bool
}

// ParseProtectionFlags decodes protection status flags
// Note: Bit mappings are estimates and may need verification with actual hardware
func ParseProtectionFlags(flags uint16) ProtectionStatus {
	return ProtectionStatus{
		OverVoltage:          (flags & 0x0001) != 0,
		UnderVoltage:         (flags & 0x0002) != 0,
		OverCurrent:          (flags & 0x0004) != 0,
		OverTemperature:      (flags & 0x0008) != 0,
		UnderTemperature:     (flags & 0x0010) != 0,
		ShortCircuit:         (flags & 0x0020) != 0,
		DischargeOverCurrent: (flags & 0x0040) != 0,
		ChargeOverCurrent:    (flags & 0x0080) != 0,
	}
}
