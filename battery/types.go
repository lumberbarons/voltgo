package battery

import "time"

// Status represents the overall battery status
type Status struct {
	Voltage     float64   // Total battery voltage in volts
	Current     float64   // Current in amperes (positive=charging, negative=discharging)
	SOC         int       // State of Charge percentage (0-100)
	SOH         int       // State of Health percentage (0-100)
	Temperature float64   // Temperature in Celsius
	CellCount   int       // Number of cells in series
	Cells       []Cell    // Individual cell data
	Capacity    Capacity  // Battery capacity information
	Cycles      int       // Charge/discharge cycle count
	UpdatedAt   time.Time // Last update timestamp
}

// Cell represents individual cell information
type Cell struct {
	Index       int     // Cell index (0-based)
	Voltage     float64 // Cell voltage in volts
	Temperature float64 // Cell temperature in Celsius (if available)
	Balanced    bool    // Whether cell is balanced
}

// Capacity represents battery capacity information
type Capacity struct {
	Rated     float64 // Rated capacity in Ah
	Remaining float64 // Remaining capacity in Ah
	Full      float64 // Full charge capacity in Ah
}

// Protection represents BMS protection status flags
type Protection struct {
	OverVoltage          bool // Cell over-voltage protection triggered
	UnderVoltage         bool // Cell under-voltage protection triggered
	OverCurrent          bool // Over-current protection triggered
	OverTemperature      bool // Over-temperature protection triggered
	UnderTemperature     bool // Under-temperature protection triggered
	ShortCircuit         bool // Short circuit protection triggered
	DischargeOverCurrent bool // Discharge over-current protection triggered
	ChargeOverCurrent    bool // Charge over-current protection triggered
}

// Info represents battery/BMS information
type Info struct {
	Model          string  // Battery model
	Manufacturer   string  // Manufacturer name
	SerialNumber   string  // Serial number
	HardwareVersion string // Hardware version
	SoftwareVersion string // Software/firmware version
	Chemistry      string  // Battery chemistry (e.g., "LiFePO4")
	NominalVoltage float64 // Nominal voltage
	RatedCapacity  float64 // Rated capacity in Ah
}

// Response represents a generic BMS response
type Response struct {
	CommandID byte      // Command ID this response is for
	Data      []byte    // Raw response data
	Timestamp time.Time // Response timestamp
}

// DeviceInfo represents discovered BLE device information
type DeviceInfo struct {
	Name    string // Device name
	Address string // BLE MAC address
	RSSI    int16  // Signal strength
}
