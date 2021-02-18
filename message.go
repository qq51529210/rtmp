package rtmp

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	CSID      uint32       // 消息属于的块流
	Timestamp uint32       // 时间戳
	TypeID    uint8        // 消息类型
	StreamID  uint32       // 消息属于的流
	Length    uint32       // 消息的长度
	Data      bytes.Buffer // 消息的数据
}

func (m *Message) Write(w io.Writer, chunkSize int) error {
	var chunk ChunkHeader
	// 第一个chunk
	chunk.CSID = m.CSID
	chunk.FMT = 0
	chunk.MessageLength = m.Length
	chunk.MessageTypeID = m.TypeID
	chunk.MessageStreamID = m.StreamID
	if m.Timestamp > MaxMessageTimestamp {
		chunk.MessageTimestamp = MaxMessageTimestamp
		chunk.ExtendedTimestamp = m.Timestamp
	} else {
		chunk.MessageTimestamp = m.Timestamp
	}
	err := chunk.Write(w)
	if err != nil {
		return err
	}
	d := m.Data.Bytes()
	n := len(d)
	if n > chunkSize {
		n = chunkSize
	}
	_, err = w.Write(d[:n])
	if err != nil {
		return err
	}
	d = d[n:]
	// 其他的chunk
	chunk.FMT = 3
	for len(d) > 0 {
		err = chunk.Write(w)
		if err != nil {
			return err
		}
		n = len(d)
		if n > chunkSize {
			n = chunkSize
		}
		_, err = w.Write(d[:n])
		if err != nil {
			return err
		}
		d = d[n:]
	}
	return nil
}

func (m *Message) PutBigEndianUint16(n uint16) {
	m.Data.WriteByte(byte(n >> 8))
	m.Data.WriteByte(byte(n))
}

func (m *Message) PutBigEndianUint32(n uint32) {
	m.Data.WriteByte(byte(n >> 24))
	m.Data.WriteByte(byte(n >> 16))
	m.Data.WriteByte(byte(n >> 8))
	m.Data.WriteByte(byte(n))
}

func (m *Message) InitControlMessage() {
	m.StreamID = ControlCommandMessageStreamID
	m.CSID = ControlMessageChunkStreamID
	m.Timestamp = 0
	m.Data.Reset()
}

func (m *Message) InitControlMessageSetBandWidth(bandWidth uint32, limit byte) {
	m.InitControlMessage()
	m.TypeID = ControlMessageSetBandWidth
	m.PutBigEndianUint32(bandWidth)
	m.Data.WriteByte(limit)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageWindowAcknowledgementSize(size uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageWindowAcknowledgementSize
	m.PutBigEndianUint32(size)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageAcknowledgement(n uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageAcknowledgement
	m.PutBigEndianUint32(n)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageAbort(csid uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageAbort
	m.PutBigEndianUint32(csid)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageSetChunkSize(size uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageSetChunkSize
	m.PutBigEndianUint32(size)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamBegin(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamBegin)
	m.PutBigEndianUint32(streamID)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamEOF(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamEOF)
	m.PutBigEndianUint32(streamID)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamDry(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamDry)
	m.PutBigEndianUint32(streamID)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageSetBufferLength(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageSetBufferLength)
	m.PutBigEndianUint32(streamID)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamIsRecorded(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamIsRecorded)
	m.PutBigEndianUint32(streamID)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessagePingRequest(timestamp uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessagePingRequest)
	m.PutBigEndianUint32(timestamp)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessagePingResponse(timestamp uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessagePingResponse)
	m.PutBigEndianUint32(timestamp)
	m.Length = uint32(m.Data.Len())
}

func (m *Message) InitCommandMessage(name string, obj ...interface{}) {
	m.StreamID = ControlCommandMessageStreamID
	m.CSID = CommandMessageChunkStreamID
	m.Timestamp = 0
	m.TypeID = CommandMessageAMF0
	m.Data.Reset()
	WriteAMF(&m.Data, name)
	for _, o := range obj {
		WriteAMF(&m.Data, o)
	}
	m.Length = uint32(m.Data.Len())
}

type MessageHandler struct {
	message         []*Message  // key:chunk stream id
	chunkHeader     ChunkHeader // chunk头
	localChunkSize  int         // 本地chunk的大小
	RemoteChunkSize int         // 对方chunk的大小
	MaxChunkSize    uint32      // 对方可以设置的最大的chunk size，为0表示不限制
	AckSize         uint32      // Window Acknowledge Size消息的值
	Ack             uint32      // Acknowledgement消息的值
	BandWidth       uint32      // Set Bandwith消息的值
	BandWidthLimit  byte        // Set Bandwith消息的值
}

func (m *MessageHandler) getMessage(csid uint32) *Message {
	for _, msg := range m.message {
		if msg.CSID == csid {
			return msg
		}
	}
	msg := new(Message)
	msg.CSID = csid
	m.message = append(m.message, msg)
	return msg
}

func (m *MessageHandler) Read(conn io.ReadWriter, handle func(*Message) error) error {
	m.localChunkSize = ChunkSize
	m.RemoteChunkSize = ChunkSize
	m.message = make([]*Message, 0)
	var n int
	var err error
	var msg *Message
	for {
		// 读取chunk header
		err = m.chunkHeader.Read(conn)
		if err != nil {
			return err
		}
		msg = m.getMessage(m.chunkHeader.CSID)
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
		if int(msg.Length) > msg.Data.Len() {
			n = int(msg.Length) - msg.Data.Len()
			if n > m.localChunkSize {
				n = m.localChunkSize
			}
			_, err = io.CopyN(&msg.Data, conn, int64(n))
			if err != nil {
				return err
			}
		} else {
			// 处理控制消息
			switch msg.TypeID {
			case ControlMessageSetChunkSize:
				err = m.handleControlMessageSetChunkSize(msg)
			case ControlMessageAbort:
				err = m.handleControlMessageAbort(msg)
			case ControlMessageAcknowledgement:
				err = m.handleControlMessageAcknowledgement(msg)
			case ControlMessageWindowAcknowledgementSize:
				err = m.handleControlMessageWindowAcknowledgementSize(msg)
			case ControlMessageSetBandWidth:
				err = m.handleControlMessageSetBandWidth(msg)
			default:
				// 上层处理
				err = handle(msg)
			}
			if err != nil {
				return err
			}
			// 发送ack
			m.Ack += msg.Length
			if m.AckSize <= m.Ack {
				msg.InitControlMessageAcknowledgement(m.Ack)
				err = msg.Write(conn, m.localChunkSize)
				if err != nil {
					return err
				}
				m.Ack = 0
			}
			// 重置数据
			msg.Data.Reset()
		}
	}
}

func (m *MessageHandler) handleControlMessageSetBandWidth(msg *Message) (err error) {
	if msg.Length != 5 {
		return fmt.Errorf("control message 'set bandwidth' invalid length <%d>", msg.Length)
	}
	data := msg.Data.Bytes()
	m.BandWidth = binary.BigEndian.Uint32(data)
	m.BandWidthLimit = data[4]
	return
}

func (m *MessageHandler) handleControlMessageWindowAcknowledgementSize(msg *Message) (err error) {
	if msg.Length != 4 {
		return fmt.Errorf("control message 'window acknowledgement size' invalid length <%d>", msg.Length)
	}
	m.AckSize = binary.BigEndian.Uint32(msg.Data.Bytes())
	return
}

func (m *MessageHandler) handleControlMessageAcknowledgement(msg *Message) (err error) {
	if msg.Length != 4 {
		return fmt.Errorf("control message 'acknowledgement' invalid length <%d>", msg.Length)
	}
	// m.Ack = binary.BigEndian.Uint32(msg.Data.Bytes())
	return
}

func (m *MessageHandler) handleControlMessageAbort(msg *Message) (err error) {
	if msg.Length != 4 {
		return fmt.Errorf("control message 'abort' invalid length <%d>", msg.Length)
	}
	csid := binary.BigEndian.Uint32(msg.Data.Bytes())
	abort := m.getMessage(csid)
	abort.Data.Reset()
	return
}

func (m *MessageHandler) handleControlMessageSetChunkSize(msg *Message) (err error) {
	if msg.Length != 4 {
		return fmt.Errorf("control message 'set chunk size' invalid length <%d>", msg.Length)
	}
	// chunk size
	size := binary.BigEndian.Uint32(msg.Data.Bytes())
	// 比设定的大
	if m.MaxChunkSize > 0 && size > m.MaxChunkSize {
		return fmt.Errorf("control message 'set chunk size' <%d> too big", size)
	}
	// 由于消息的最大长度为 16777215(0xFFFFFF)
	if size > 0xFFFFFF {
		size = 0xFFFFFF
	}
	m.localChunkSize = int(size)
	return
}
