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
	length    uint32       // 消息的长度
	Data      bytes.Buffer // 消息的数据
}

func WriteMessage(w io.Writer, chunkSize int, chunk *ChunkHeader, data []byte) error {
	// 第一个chunk
	chunk.FMT = 0
	chunk.MessageLength = uint32(len(data))
	err := chunk.Write(w)
	if err != nil {
		return err
	}
	n := len(data)
	if n > chunkSize {
		n = chunkSize
	}
	_, err = w.Write(data[:n])
	if err != nil {
		return err
	}
	data = data[n:]
	// 其他的chunk
	chunk.FMT = 3
	for len(data) > 0 {
		err = chunk.Write(w)
		if err != nil {
			return err
		}
		n = len(data)
		if n > chunkSize {
			n = chunkSize
		}
		_, err = w.Write(data[:n])
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (m *Message) Write(w io.Writer, chunkSize int) error {
	var chunk ChunkHeader
	// 第一个chunk
	chunk.CSID = m.CSID
	chunk.FMT = 0
	chunk.MessageLength = uint32(m.Data.Len())
	chunk.MessageTypeID = m.TypeID
	chunk.MessageStreamID = m.StreamID
	chunk.MessageTimestamp = m.Timestamp
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
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageWindowAcknowledgementSize(size uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageWindowAcknowledgementSize
	m.PutBigEndianUint32(size)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageAcknowledgement(n uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageAcknowledgement
	m.PutBigEndianUint32(n)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageAbort(csid uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageAbort
	m.PutBigEndianUint32(csid)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitControlMessageSetChunkSize(size uint32) {
	m.InitControlMessage()
	m.TypeID = ControlMessageSetChunkSize
	m.PutBigEndianUint32(size)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamBegin(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamBegin)
	m.PutBigEndianUint32(streamID)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamEOF(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamEOF)
	m.PutBigEndianUint32(streamID)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamDry(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamDry)
	m.PutBigEndianUint32(streamID)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageSetBufferLength(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageSetBufferLength)
	m.PutBigEndianUint32(streamID)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessageStreamIsRecorded(streamID uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessageStreamIsRecorded)
	m.PutBigEndianUint32(streamID)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessagePingRequest(timestamp uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessagePingRequest)
	m.PutBigEndianUint32(timestamp)
	m.length = uint32(m.Data.Len())
}

func (m *Message) InitUserControlMessagePingResponse(timestamp uint32) {
	m.InitControlMessage()
	m.TypeID = UserControlMessage
	m.PutBigEndianUint16(UserControlMessagePingResponse)
	m.PutBigEndianUint32(timestamp)
	m.length = uint32(m.Data.Len())
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
	m.length = uint32(m.Data.Len())
}

type MessageReader struct {
	message         []*Message  // key:chunk stream id
	chunkHeader     ChunkHeader // chunk头
	localChunkSize  int         // 本地chunk的大小
	RemoteChunkSize int         // 对方chunk的大小
	MaxChunkSize    uint32      // 对方可以设置的最大的chunk size，为0表示不限制
	AckSize         uint32      // Window Acknowledge Size消息的值
	Ack             uint32      // Acknowledgement消息的值
	BandWidth       uint32      // Set Bandwith消息的值
	BandWidthLimit  byte        // Set Bandwith消息的值
	ack             Message
}

func (m *MessageReader) Init() {
	m.AckSize = 500 * 1024
	m.localChunkSize = ChunkSize
	m.RemoteChunkSize = ChunkSize
	m.message = make([]*Message, 0)
	m.ack.InitControlMessage()
	m.ack.TypeID = ControlMessageAcknowledgement
	m.ack.length = 4
}

func (m *MessageReader) getMessage(csid uint32) *Message {
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

// 读取一个完整的消息，
func (m *MessageReader) Read(conn io.ReadWriter) (*Message, error) {
	var n int
	var err error
	var msg *Message
	for {
		// 读取chunk header
		err = m.chunkHeader.Read(conn)
		if err != nil {
			return nil, err
		}
		// 消息
		msg = m.getMessage(m.chunkHeader.CSID)
		if msg.length == uint32(msg.Data.Len()) {
			msg.Data.Reset()
		}
		// 消息头
		switch m.chunkHeader.FMT {
		case 0:
			msg.Timestamp = m.chunkHeader.MessageTimestamp
			msg.length = m.chunkHeader.MessageLength
			msg.TypeID = m.chunkHeader.MessageTypeID
			msg.StreamID = m.chunkHeader.MessageStreamID
		case 1:
			if m.chunkHeader.MessageTimestamp > MaxMessageTimestamp {
				msg.Timestamp = m.chunkHeader.MessageTimestamp
			} else {
				msg.Timestamp += m.chunkHeader.MessageTimestamp
			}
			msg.length = m.chunkHeader.MessageLength
			msg.TypeID = m.chunkHeader.MessageTypeID
		case 2:
			if m.chunkHeader.MessageTimestamp > MaxMessageTimestamp {
				msg.Timestamp = m.chunkHeader.MessageTimestamp
			} else {
				msg.Timestamp += m.chunkHeader.MessageTimestamp
			}
		default:
		}
		// 消息数据
		if int(msg.length) > msg.Data.Len() {
			n = int(msg.length) - msg.Data.Len()
			if n > m.localChunkSize {
				n = m.localChunkSize
			}
			_, err = io.CopyN(&msg.Data, conn, int64(n))
			if err != nil {
				return nil, err
			}
			if int(msg.length) > msg.Data.Len() {
				continue
			}
		}
		// 发送ack
		m.Ack += msg.length
		if m.AckSize <= m.Ack {
			m.ack.Data.Reset()
			m.ack.PutBigEndianUint32(m.Ack)
			err = m.ack.Write(conn, m.localChunkSize)
			if err != nil {
				return nil, err
			}
			m.Ack = 0
		}
		switch msg.TypeID {
		case ControlMessageSetChunkSize:
			err = m.HandleControlMessageSetChunkSize(msg)
		case ControlMessageAbort:
			err = m.HandleControlMessageAbort(msg)
		case ControlMessageAcknowledgement:
			err = m.HandleControlMessageAcknowledgement(msg)
		case ControlMessageWindowAcknowledgementSize:
			err = m.HandleControlMessageWindowAcknowledgementSize(msg)
		case ControlMessageSetBandWidth:
			err = m.HandleControlMessageSetBandWidth(msg)
		default:
			return msg, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func (m *MessageReader) HandleControlMessage(msg *Message) (err error) {
	switch msg.TypeID {
	case ControlMessageSetChunkSize:
		err = m.HandleControlMessageSetChunkSize(msg)
	case ControlMessageAbort:
		err = m.HandleControlMessageAbort(msg)
	case ControlMessageAcknowledgement:
		err = m.HandleControlMessageAcknowledgement(msg)
	case ControlMessageWindowAcknowledgementSize:
		err = m.HandleControlMessageWindowAcknowledgementSize(msg)
	case ControlMessageSetBandWidth:
		err = m.HandleControlMessageSetBandWidth(msg)
	}
	return
}

func (m *MessageReader) HandleControlMessageSetBandWidth(msg *Message) (err error) {
	if msg.length != 5 {
		return fmt.Errorf("control message 'set bandwidth' invalid length <%d>", msg.length)
	}
	data := msg.Data.Bytes()
	m.BandWidth = binary.BigEndian.Uint32(data)
	m.BandWidthLimit = data[4]
	return
}

func (m *MessageReader) HandleControlMessageWindowAcknowledgementSize(msg *Message) (err error) {
	if msg.length != 4 {
		return fmt.Errorf("control message 'window acknowledgement size' invalid length <%d>", msg.length)
	}
	m.AckSize = binary.BigEndian.Uint32(msg.Data.Bytes())
	return
}

func (m *MessageReader) HandleControlMessageAcknowledgement(msg *Message) (err error) {
	if msg.length != 4 {
		return fmt.Errorf("control message 'acknowledgement' invalid length <%d>", msg.length)
	}
	// m.Ack = binary.BigEndian.Uint32(msg.Data.Bytes())
	return
}

func (m *MessageReader) HandleControlMessageAbort(msg *Message) (err error) {
	if msg.length != 4 {
		return fmt.Errorf("control message 'abort' invalid length <%d>", msg.length)
	}
	csid := binary.BigEndian.Uint32(msg.Data.Bytes())
	abort := m.getMessage(csid)
	abort.Data.Reset()
	return
}

func (m *MessageReader) HandleControlMessageSetChunkSize(msg *Message) (err error) {
	if msg.length != 4 {
		return fmt.Errorf("control message 'set chunk size' invalid length <%d>", msg.length)
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
