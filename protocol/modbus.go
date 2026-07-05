package protocol

import (
	"encoding/binary"
	"fmt"
)

// The batteries speak Modbus RTU framed over BLE GATT writes. A request is a
// standard Modbus read-holding-registers frame written to the write
// characteristic; the response arrives as a single notification on the notify
// characteristic. Frames with an invalid CRC are silently ignored by the BMS.
const (
	DefaultSlaveAddr = 0x01

	FuncReadHoldingRegisters = 0x03

	// Modbus exception responses set the high bit of the function code.
	exceptionFlag = 0x80

	// Request: addr + func + start(2) + count(2) + crc(2)
	requestLength = 8
	// Response: addr + func + bytecount + payload + crc(2)
	minResponseLength = 5
)

var (
	ErrShortFrame    = fmt.Errorf("frame too short")
	ErrCRCMismatch   = fmt.Errorf("CRC mismatch")
	ErrWrongSlave    = fmt.Errorf("unexpected slave address")
	ErrWrongFunction = fmt.Errorf("unexpected function code")
)

// CRC16 computes the CRC-16/MODBUS checksum (poly 0xA001, init 0xFFFF).
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// AppendCRC returns frame with its CRC-16/MODBUS appended (low byte first,
// per Modbus RTU convention).
func AppendCRC(frame []byte) []byte {
	crc := CRC16(frame)
	return append(frame, byte(crc&0xFF), byte(crc>>8))
}

// VerifyCRC reports whether the trailing two bytes of frame are a valid
// CRC-16/MODBUS of the preceding bytes.
func VerifyCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	crc := CRC16(frame[:len(frame)-2])
	return frame[len(frame)-2] == byte(crc&0xFF) && frame[len(frame)-1] == byte(crc>>8)
}

// NewReadRequest builds a read-holding-registers request frame.
// Register addresses and counts are big-endian per Modbus RTU.
func NewReadRequest(slaveAddr byte, startReg, count uint16) []byte {
	frame := make([]byte, 6, requestLength)
	frame[0] = slaveAddr
	frame[1] = FuncReadHoldingRegisters
	binary.BigEndian.PutUint16(frame[2:4], startReg)
	binary.BigEndian.PutUint16(frame[4:6], count)
	return AppendCRC(frame)
}

// NewReadResponse builds a read-holding-registers response frame carrying
// the given register values. This is the encoder counterpart of
// ParseReadResponse, for device emulators and round-trip tests.
func NewReadResponse(slaveAddr byte, regs []uint16) []byte {
	frame := make([]byte, 3, 3+len(regs)*2+2)
	frame[0] = slaveAddr
	frame[1] = FuncReadHoldingRegisters
	frame[2] = byte(len(regs) * 2)
	for _, r := range regs {
		frame = binary.BigEndian.AppendUint16(frame, r)
	}
	return AppendCRC(frame)
}

// NewExceptionResponse builds a Modbus exception response frame for the
// given function and exception code, for device emulators.
func NewExceptionResponse(slaveAddr, function, code byte) []byte {
	return AppendCRC([]byte{slaveAddr, function | exceptionFlag, code})
}

// ModbusError is a Modbus exception response from the device.
type ModbusError struct {
	Function byte
	Code     byte
}

func (e *ModbusError) Error() string {
	return fmt.Sprintf("modbus exception: function 0x%02x, code 0x%02x", e.Function, e.Code)
}

// ParseReadResponse validates a read-holding-registers response frame and
// returns the register values.
func ParseReadResponse(frame []byte, slaveAddr byte) ([]uint16, error) {
	if len(frame) < minResponseLength {
		return nil, fmt.Errorf("%w: %d bytes", ErrShortFrame, len(frame))
	}
	if !VerifyCRC(frame) {
		return nil, fmt.Errorf("%w: frame %x", ErrCRCMismatch, frame)
	}
	if frame[0] != slaveAddr {
		return nil, fmt.Errorf("%w: got 0x%02x, want 0x%02x", ErrWrongSlave, frame[0], slaveAddr)
	}
	if frame[1]&exceptionFlag != 0 {
		return nil, &ModbusError{Function: frame[1] &^ exceptionFlag, Code: frame[2]}
	}
	if frame[1] != FuncReadHoldingRegisters {
		return nil, fmt.Errorf("%w: got 0x%02x, want 0x%02x", ErrWrongFunction, frame[1], FuncReadHoldingRegisters)
	}

	byteCount := int(frame[2])
	if byteCount%2 != 0 || len(frame) != 3+byteCount+2 {
		return nil, fmt.Errorf("byte count %d inconsistent with frame length %d", byteCount, len(frame))
	}

	regs := make([]uint16, byteCount/2)
	for i := range regs {
		regs[i] = binary.BigEndian.Uint16(frame[3+i*2 : 5+i*2])
	}
	return regs, nil
}
