package protocol

// Command IDs based on Voltgo app response mappings
const (
	// Response command types
	ResponseType02 = 0x02 // Maps to command type 3
	ResponseType03 = 0x03 // Maps to command type 11 (0x0B)
	ResponseType04 = 0x04 // Maps to command type 5
	ResponseType05 = 0x05 // Maps to command type 13 (0x0D)
	ResponseType06 = 0x06 // Maps to command type 6
	ResponseType07 = 0x07 // Maps to command type 7
	ResponseType09 = 0x09 // Maps to command type 9
	ResponseType0A = 0x0A // Maps to command type 10
	ResponseType0B = 0x0B // Maps to command type 4
	ResponseType0C = 0x0C // Maps to command type 12
	ResponseType0D = 0x0D // Maps to command type 5
)

// Command represents a BMS command
type Command struct {
	ID   byte
	Data []byte
}

// NewCommand creates a new command
func NewCommand(id byte, data []byte) *Command {
	return &Command{
		ID:   id,
		Data: data,
	}
}

// ToPacket converts the command to a packet
func (c *Command) ToPacket() *Packet {
	return NewPacket(c.ID, c.Data)
}
