package rtmp

import (
	"io"
)

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

type MessageReader struct {
	chunkHeader ChunkHeader // chunk头
	chunkSize   int         // chunk的大小
	message     []*Message  // key:chunk stream id
	// MaxChunkSize              int         // 设置的最大chunk，防止chunk缓存过大
	// acknowledgement           uint32      // acknowledgement消息的值
	// windowAcknowledgementSize uint32      // window acknowledgement size的值
	// bandWidth                 uint32      // set bandWidth消息的值
	// bandWidthLimit            byte        // set bandWidth消息的值
}

func (m *MessageReader) getMessage() *Message {
	for _, msg := range m.message {
		if msg.CSID == m.chunkHeader.CSID {
			return msg
		}
	}
	msg := new(Message)
	msg.CSID = m.chunkHeader.CSID
	m.message = append(m.message, msg)
	return msg
}

func (m *MessageReader) SetChunkSize(size int) {
	if size > 0 {
		m.chunkSize = size
	} else {
		m.chunkSize = ChunkSize
	}
}

func (m *MessageReader) ReadLoop(conn io.ReadWriter, handle func(*Message) error) (err error) {
	m.chunkSize = ChunkSize
	m.message = make([]*Message, 0)
	var n, i1, i2 int
	var msg *Message
	for {
		// 读取chunk header
		err = m.chunkHeader.Read(conn)
		if err != nil {
			return err
		}
		msg = m.getMessage()
		// 消息头
		switch m.chunkHeader.FMT {
		case 0:
			msg.Timestamp = m.chunkHeader.MessageTimestamp
			msg.Length = m.chunkHeader.MessageLength
			msg.TypeID = m.chunkHeader.MessageTypeID
			msg.StreamID = m.chunkHeader.MessageStreamID
		case 1:
			if m.chunkHeader.MessageTimestamp == MaxMessageTimestamp {
				msg.Timestamp = m.chunkHeader.ExtendedTimestamp
			} else {
				msg.Timestamp += m.chunkHeader.MessageTimestamp
			}
			msg.Length = m.chunkHeader.MessageLength
			msg.TypeID = m.chunkHeader.MessageTypeID
		case 2:
			if m.chunkHeader.MessageTimestamp == MaxMessageTimestamp {
				msg.Timestamp = m.chunkHeader.ExtendedTimestamp
			} else {
				msg.Timestamp += m.chunkHeader.MessageTimestamp
			}
		default:
		}
		// 消息数据
		n = int(msg.Length) - len(msg.Data)
		if n > 0 {
			if n > m.chunkSize {
				n = m.chunkSize
			}
			i1 = len(msg.Data)
			i2 = i1 + n
			if cap(msg.Data) >= i2 {
				msg.Data = msg.Data[:i2]
			} else {
				msg.Data = append(msg.Data, make([]byte, i2-cap(msg.Data))...)
			}
			_, err = io.ReadFull(conn, msg.Data[i1:i2])
			if err != nil {
				return
			}
		} else {
			// 处理消息
			err = handle(msg)
			if err != nil {
				return
			}
			msg.Data = msg.Data[:0]
		}
	}
}
