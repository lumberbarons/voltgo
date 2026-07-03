package protocol

import (
	"fmt"
	"strings"
)

// Register map for the status block (holding registers 0-40). Derived from
// live reads of ZT-25.6V100Ah batteries; entries marked "unverified" have not
// been observed at a non-trivial value yet.
const (
	RegVoltage      = 0  // pack voltage, 0.01 V
	RegCurrent      = 1  // pack current, int16, 0.1 A (sign/scale unverified: always 0 at idle so far)
	RegCellBase     = 2  // cell voltages, 1 mV, 16 slots (regs 2-17)
	RegTempBase     = 18 // temperature sensors, int16 °C, 3 slots (regs 18-20)
	RegSOC          = 21 // state of charge, % (regs 21/24 track together; 21 assumed SOC)
	RegSOH          = 22 // state of health, % (regs 22/23 track together; 22 assumed SOH)
	RegCellCount    = 36 // number of cells in series
	RegFullCapacity = 37 // full capacity, 0.1 Ah

	// StatusRegisterCount is the number of registers the Voltgo app requests
	// for a status poll (read 0x29 registers from address 0).
	StatusRegisterCount = 41

	// DeviceInfoStart/Count cover the ASCII device-info block (model,
	// hardware version, manufacture date as NUL-padded strings).
	DeviceInfoStart = 105
	DeviceInfoCount = 32

	maxCellSlots = 16
	tempSensors  = 3
)

// BMSInfo contains battery management system data parsed from the status
// register block.
type BMSInfo struct {
	Voltage        float64   // pack voltage in volts
	Current        float64   // pack current in amps (positive=charge, negative=discharge)
	CellVoltages   []float64 // per-cell voltages in volts
	CellCount      int       // number of cells in series
	SOC            int       // state of charge, percent
	SOH            int       // state of health, percent
	Temperatures   []int     // temperature sensor readings in °C
	FullCapacityAh float64   // full capacity in Ah
	RawRegisters   []uint16  // complete register block for fields not yet mapped
}

// ParseBMSInfo parses the status register block (registers 0-40).
func ParseBMSInfo(regs []uint16) (*BMSInfo, error) {
	if len(regs) < RegFullCapacity+1 {
		return nil, fmt.Errorf("register block too short: %d registers, need %d", len(regs), RegFullCapacity+1)
	}

	info := &BMSInfo{
		Voltage:        float64(regs[RegVoltage]) / 100.0,
		Current:        float64(int16(regs[RegCurrent])) / 10.0,
		SOC:            int(regs[RegSOC]),
		SOH:            int(regs[RegSOH]),
		FullCapacityAh: float64(regs[RegFullCapacity]) / 10.0,
		RawRegisters:   append([]uint16(nil), regs...),
	}

	cellCount := int(regs[RegCellCount])
	if cellCount < 1 || cellCount > maxCellSlots {
		return nil, fmt.Errorf("implausible cell count: %d", cellCount)
	}
	info.CellCount = cellCount
	info.CellVoltages = make([]float64, cellCount)
	for i := range cellCount {
		info.CellVoltages[i] = float64(regs[RegCellBase+i]) / 1000.0
	}

	info.Temperatures = make([]int, tempSensors)
	for i := range tempSensors {
		info.Temperatures[i] = int(int16(regs[RegTempBase+i]))
	}

	return info, nil
}

// DeviceInfo contains the ASCII identity strings from the device-info
// register block.
type DeviceInfo struct {
	Strings []string // NUL-separated ASCII fields, e.g. model, hw version, date
}

// ParseDeviceInfo extracts printable ASCII fields from the device-info
// register block (registers 105+).
func ParseDeviceInfo(regs []uint16) *DeviceInfo {
	raw := make([]byte, 0, len(regs)*2)
	for _, r := range regs {
		raw = append(raw, byte(r>>8), byte(r&0xFF))
	}

	var fields []string
	for _, part := range strings.FieldsFunc(string(raw), func(r rune) bool { return r == 0 }) {
		if isPrintableASCII(part) && len(part) > 1 {
			fields = append(fields, part)
		}
	}
	return &DeviceInfo{Strings: fields}
}

func isPrintableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			return false
		}
	}
	return true
}
