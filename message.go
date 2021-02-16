package rtmp

const (
	ControlMessageSetChunkSize              = 1
	ControlMessageAbort                     = 2
	ControlMessageAcknowledgement           = 3
	ControlMessageWindowAcknowledgementSize = 5
	ControlMessageSetBandWidth              = 6

	CommandMessageAMF0 = 20
	CommandMessageAMF3 = 17

	DataMessageAMF0 = 18
	DataMessageAMF3 = 15

	SharedObjectMessageAMF0 = 19
	SharedObjectMessageAMF3 = 16

	AudioMessage = 8
	VideoMessage = 9

	AggregateMessage = 22

	UserControlMessage = 4

	UserControlMessageStreamBegin      = 0
	UserControlMessageStreamEOF        = 1
	UserControlMessageStreamDry        = 2
	UserControlMessageSetBufferLength  = 3
	UserControlMessageStreamIsRecorded = 4
	UserControlMessagePingRequest      = 6
	UserControlMessagePingResponse     = 7

	ControlCommandMessageStreamID = 0
	ControlMessageChunkStreamID   = 2
	CommandMessageChunkStreamID   = 3
)

type Message struct {
	CSID      uint32 // 消息属于的块流
	Timestamp uint32 // 时间戳
	TypeID    uint8  // 消息类型
	StreamID  uint32 // 消息属于的流
	Length    uint32 // 消息的长度
	Data      []byte // 消息的数据
}

func (m *Message) IsComplete() bool {
	return len(m.Data) < int(m.Length)
}

func (m *Message) Invalid() bool {
	return m.Length == 0
}
