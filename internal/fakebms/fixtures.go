package fakebms

// LiveStatusRegisters is the status block (registers 0-40) captured from a
// ZT-25.6V100Ah battery: 8S LiFePO4, idle at ~100% SOC, 26.83V pack voltage.
// It matches the frame corpus in protocol/testdata.
var LiveStatusRegisters = []uint16{
	2683,                                           // 0: voltage (26.83V)
	0,                                              // 1: current
	3350, 3352, 3353, 3356, 3359, 3356, 3355, 3355, // 2-9: cells 1-8
	0, 0, 0, 0, 0, 0, 0, 0, // 10-17: unused cell slots
	27, 27, 27, // 18-20: temperatures
	99, 100, 100, 99, // 21-24: SOC, SOH, +2 unmapped
	0, 0, 0, 0, // 25-28
	2, 0, // 29-30
	0x1575, 0x2a00, 0x1b1b, // 31-33
	0, 0, // 34-35
	8,       // 36: cell count
	1000,    // 37: full capacity (100.0Ah)
	0, 0, 0, // 38-40
}

// LiveDeviceInfoRegisters is the ASCII device-info block (registers 105-136)
// from the same battery: "TC", "-8S100-V1.0", serial/date block.
var LiveDeviceInfoRegisters = []uint16{
	0x5443, 0x0000, 0x2d38, 0x5331, 0x3030, 0x2d56, 0x312e, 0x3000,
	0x0000, 0x0000, 0x0000, 0x0000, 0x5a30, 0x3154, 0x3230, 0x3230,
	0x3234, 0x2d30, 0x312d, 0x3131, 0x0000, 0x0000, 0x0000, 0x0000,
	0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
}
