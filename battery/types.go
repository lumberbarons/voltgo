package battery

import "time"

// Status represents the overall battery status
type Status struct {
	Voltage      float64   // Total battery voltage in volts
	Current      float64   // Current in amperes (positive=charging, negative=discharging)
	SOC          int       // State of Charge percentage (0-100)
	SOH          int       // State of Health percentage (0-100)
	Temperature  float64   // Average temperature in Celsius
	Temperatures []int     // Individual temperature sensor readings in Celsius
	CellCount    int       // Number of cells in series
	Cells        []Cell    // Individual cell data
	UpdatedAt    time.Time // Last update timestamp
}

// Cell represents individual cell information
type Cell struct {
	Index   int     // Cell index (0-based)
	Voltage float64 // Cell voltage in volts
}

// Info represents battery/BMS information
type Info struct {
	Chemistry      string   // Battery chemistry (e.g., "LiFePO4")
	NominalVoltage float64  // Nominal voltage
	CapacityAh     float64  // Full capacity in amp-hours
	DeviceStrings  []string // ASCII identity strings from the device (model, hw version, date)
}

// DeviceInfo represents discovered BLE device information
type DeviceInfo struct {
	Name    string // Device name
	Address string // BLE address (MAC address, or CoreBluetooth UUID on macOS)
	RSSI    int16  // Signal strength
}

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

// DeviceIdentity contains the ASCII identity strings from the device-info
// register block.
type DeviceIdentity struct {
	Strings []string // NUL-separated ASCII fields, e.g. model, hw version, date
}
