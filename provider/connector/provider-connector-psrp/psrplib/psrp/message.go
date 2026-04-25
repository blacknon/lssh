package psrp

import (
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
)

const headerLength = 40

type Destination uint32

const (
	DestinationClient Destination = 0x00000001
	DestinationServer Destination = 0x00000002
)

type MessageType uint32

const (
	MessageSessionCapability     MessageType = 0x00010002
	MessageInitRunspacePool      MessageType = 0x00010004
	MessagePublicKey             MessageType = 0x00010005
	MessageEncryptedSessionKey   MessageType = 0x00010006
	MessagePublicKeyRequest      MessageType = 0x00010007
	MessageConnectRunspacePool   MessageType = 0x00010008
	MessageRunspacePoolInitData  MessageType = 0x0002100B
	MessageSetMaxRunspaces       MessageType = 0x00021002
	MessageSetMinRunspaces       MessageType = 0x00021003
	MessageRunspaceAvailability  MessageType = 0x00021004
	MessageRunspacePoolState     MessageType = 0x00021005
	MessageCreatePipeline        MessageType = 0x00021006
	MessageGetAvailableRunspaces MessageType = 0x00021007
	MessageUserEvent             MessageType = 0x00021008
	MessageApplicationPrivate    MessageType = 0x00021009
	MessageGetCommandMetadata    MessageType = 0x0002100A
	MessageRunspacePoolHostCall  MessageType = 0x00021100
	MessageRunspacePoolHostResp  MessageType = 0x00021101
	MessagePipelineInput         MessageType = 0x00041002
	MessageEndOfPipelineInput    MessageType = 0x00041003
	MessagePipelineOutput        MessageType = 0x00041004
	MessageErrorRecord           MessageType = 0x00041005
	MessagePipelineState         MessageType = 0x00041006
	MessageDebugRecord           MessageType = 0x00041007
	MessageVerboseRecord         MessageType = 0x00041008
	MessageWarningRecord         MessageType = 0x00041009
	MessageProgressRecord        MessageType = 0x00041010
	MessageInformationRecord     MessageType = 0x00041011
	MessagePipelineHostCall      MessageType = 0x00041100
	MessagePipelineHostResponse  MessageType = 0x00041101
)

type Message struct {
	Destination    Destination
	Type           MessageType
	RunspacePoolID uuid.UUID
	PipelineID     uuid.UUID
	Data           []byte
}

func EncodeMessage(message Message) []byte {
	buf := make([]byte, headerLength+len(message.Data))

	binary.LittleEndian.PutUint32(buf[0:4], uint32(message.Destination))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(message.Type))
	copy(buf[8:24], encodeGUIDPacket(message.RunspacePoolID))
	copy(buf[24:40], encodeGUIDPacket(message.PipelineID))
	copy(buf[40:], message.Data)

	return buf
}

func DecodeMessage(raw []byte) (Message, error) {
	if len(raw) < headerLength {
		return Message{}, fmt.Errorf("psrp message too short: %d", len(raw))
	}

	runspaceID := decodeGUIDPacket(raw[8:24])
	pipelineID := decodeGUIDPacket(raw[24:40])

	return Message{
		Destination:    Destination(binary.LittleEndian.Uint32(raw[0:4])),
		Type:           MessageType(binary.LittleEndian.Uint32(raw[4:8])),
		RunspacePoolID: runspaceID,
		PipelineID:     pipelineID,
		Data:           append([]byte(nil), raw[40:]...),
	}, nil
}

func encodeGUIDPacket(id uuid.UUID) []byte {
	raw := make([]byte, 16)
	binary.LittleEndian.PutUint32(raw[0:4], binary.BigEndian.Uint32(id[0:4]))
	binary.LittleEndian.PutUint16(raw[4:6], binary.BigEndian.Uint16(id[4:6]))
	binary.LittleEndian.PutUint16(raw[6:8], binary.BigEndian.Uint16(id[6:8]))
	copy(raw[8:16], id[8:16])
	return raw
}

func decodeGUIDPacket(raw []byte) uuid.UUID {
	if len(raw) < 16 {
		return uuid.Nil
	}

	var id uuid.UUID
	binary.BigEndian.PutUint32(id[0:4], binary.LittleEndian.Uint32(raw[0:4]))
	binary.BigEndian.PutUint16(id[4:6], binary.LittleEndian.Uint16(raw[4:6]))
	binary.BigEndian.PutUint16(id[6:8], binary.LittleEndian.Uint16(raw[6:8]))
	copy(id[8:16], raw[8:16])
	return id
}
