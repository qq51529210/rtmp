package rtmp

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Conn struct {
	conn                  io.ReadWriter
	newMessage            func() *Message
	readChunkHeader       ChunkHeader
	writeChunkHeader      ChunkHeader
	readChunkSize         uint32
	writeChunkSize        uint32
	readMessage           map[uint32]*Message // 所有正在读取的消息
	readControlMessage    Message             // 读取的控制消息
	lastStreamID          uint32              // 上一个消息
	lastTimestamp         uint32              // 上一个消息
	lastLength            uint32              // 上一个消息
	ackMessageData        [4]byte             // ack消息的缓存
	windowAcknowledgeSize uint32              // Set Acknowledge Size消息的值
	acknowledgement       uint32              // 收到的消息数据的大小
	bandWidth             uint32              // Set BandWidth消息的值
	bandWidthLimit        byte                // Set BandWidth消息的值
}

func NewConn(conn io.ReadWriter, newMessage func() *Message) *Conn {
	c := new(Conn)
	c.conn = conn
	c.newMessage = newMessage
	c.readChunkSize = ChunkSize
	c.writeChunkSize = ChunkSize
	c.readMessage = make(map[uint32]*Message)
	c.windowAcknowledgeSize = 500 * 1024
	return c
}

// 返回消息缓存
func (c *Conn) GetMessages() map[uint32]*Message {
	return c.readMessage
}

// WriteMessage的拆分函数
func (c *Conn) writeChunk(data []byte) ([]byte, error) {
	err := c.writeChunkHeader.Write(c.conn)
	if err != nil {
		return data, err
	}
	n := uint32(len(data))
	if n > c.writeChunkSize {
		n = c.writeChunkSize
	}
	_, err = c.conn.Write(data[:n])
	if err != nil {
		return data, err
	}
	return data[n:], nil
}

// 写入一个消息
func (c *Conn) WriteMessage(typeId uint8, streamId, timestamp, extTimestamp, csid uint32, data []byte) (err error) {
	// 第一个chunk
	c.writeChunkHeader.FMT = 0
	c.writeChunkHeader.CSID = csid
	c.writeChunkHeader.MessageTypeID = typeId
	c.writeChunkHeader.MessageStreamID = streamId
	c.writeChunkHeader.MessageTimestamp = timestamp
	c.writeChunkHeader.ExtendedTimestamp = extTimestamp
	c.writeChunkHeader.MessageLength = uint32(len(data))
	data, err = c.writeChunk(data)
	if err != nil {
		return err
	}
	// 其他的chunk
	c.writeChunkHeader.FMT = 3
	for len(data) > 0 {
		data, err = c.writeChunk(data)
		if err != nil {
			return err
		}
	}
	return nil
}

// 读取一个完整的消息并返回，Message使用后可以放回sync.Pool。内部已经处理ControlMessageXXX（1，2，3，5，6）。
// conn最好是一个bufio.Reader。
// newMessage是获取Message实例的函数，可以是一个sync.Pool。
func (c *Conn) ReadMessage() (*Message, error) {
	var n uint32
	var msg *Message
	var err error
	var ok bool
	for {
		// chunk header
		err = c.readChunkHeader.Read(c.conn)
		if err != nil {
			return nil, err
		}
		if c.readChunkHeader.CSID == ControlMessageChunkStreamID {
			msg = &c.readControlMessage
		} else {
			msg, ok = c.readMessage[c.readChunkHeader.CSID]
			if !ok {
				msg = c.newMessage()
				msg.Data.Reset()
			}
		}
		switch c.readChunkHeader.FMT {
		case 0:
			msg.Timestamp = c.readChunkHeader.MessageTimestamp
			if c.readChunkHeader.MessageTimestamp >= MaxMessageTimestamp {
				msg.Timestamp = c.readChunkHeader.ExtendedTimestamp
			}
			msg.Length = c.readChunkHeader.MessageLength
			msg.TypeID = c.readChunkHeader.MessageTypeID
			msg.StreamID = c.readChunkHeader.MessageStreamID
		case 1:
			if c.readChunkHeader.MessageTimestamp >= MaxMessageTimestamp {
				msg.Timestamp = c.readChunkHeader.ExtendedTimestamp
			} else {
				msg.Timestamp += c.readChunkHeader.MessageTimestamp
			}
			msg.Length = c.readChunkHeader.MessageLength
			msg.TypeID = c.readChunkHeader.MessageTypeID
		case 2:
			if c.readChunkHeader.MessageTimestamp >= MaxMessageTimestamp {
				msg.Timestamp = c.readChunkHeader.ExtendedTimestamp
			} else {
				msg.Timestamp += c.readChunkHeader.MessageTimestamp
			}
		default:
			// 新的消息，那么以上一个消息作为模板
			if !ok {
				msg.StreamID = c.lastStreamID
				msg.Length = c.lastLength
				msg.Timestamp = c.lastTimestamp
			}
		}
		// chunk data
		if int(msg.Length) > msg.Data.Len() {
			n = msg.Length - uint32(msg.Data.Len())
			if n > c.readChunkSize {
				n = c.readChunkSize
			}
			_, err = io.CopyN(&msg.Data, c.conn, int64(n))
			if err != nil {
				return nil, err
			}
			// 读完整个消息了
			if int(msg.Length) <= msg.Data.Len() {
				// 处理控制消息
				switch msg.TypeID {
				case ControlMessageSetBandWidth:
					err = c.handleControlMessageSetBandWidth(msg)
				case ControlMessageWindowAcknowledgementSize:
					err = c.handleControlMessageWindowAcknowledgementSize(msg)
				case ControlMessageAcknowledgement:
					err = c.handleControlMessageAcknowledgement(msg)
				case ControlMessageAbort:
					err = c.handleControlMessageAbort(msg)
				case ControlMessageSetChunkSize:
					err = c.handleControlMessageSetChunkSize(msg)
				default:
					// 记录
					c.lastStreamID = msg.StreamID
					c.lastLength = msg.Length
					c.lastTimestamp = msg.Timestamp
					// 返回
					if ok {
						delete(c.readMessage, c.readChunkHeader.CSID)
					}
					return msg, nil
				}
				if err != nil {
					return nil, err
				}
			} else {
				// 没读完，先缓存
				if !ok {
					c.readMessage[c.readChunkHeader.CSID] = msg
				}
			}
		}
	}
}

// 检查是否需要发送ControlMessageAcknowledgement消息
func (c *Conn) CheckWriteControlMessageAcknowledgement() error {
	if c.windowAcknowledgeSize >= c.acknowledgement {
		binary.BigEndian.PutUint32(c.ackMessageData[:], c.acknowledgement)
		c.acknowledgement = 0
		return c.WriteMessage(ControlMessageAcknowledgement, ControlCommandMessageStreamID,
			ControlMessageChunkStreamID, 0, ControlMessageChunkStreamID, c.ackMessageData[:])
	}
	return nil
}

func (c *Conn) handleControlMessageSetBandWidth(msg *Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 5 {
		return fmt.Errorf("control message 'set bandwidth' invalid length <%d>", len(data))
	}
	c.bandWidth = binary.BigEndian.Uint32(data)
	c.bandWidthLimit = data[4]
	return
}

func (c *Conn) handleControlMessageWindowAcknowledgementSize(msg *Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'window acknowledgement size' invalid length <%d>", len(data))
	}
	c.windowAcknowledgeSize = binary.BigEndian.Uint32(data)
	return
}

func (c *Conn) handleControlMessageAcknowledgement(msg *Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'acknowledgement' invalid length <%d>", len(data))
	}
	// m.Ack = binary.BigEndian.Uint32(data)
	return
}

func (c *Conn) handleControlMessageAbort(msg *Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'abort' invalid length <%d>", len(data))
	}
	csid := binary.BigEndian.Uint32(data)
	// 这个没发回收了
	delete(c.readMessage, csid)
	return
}

func (c *Conn) handleControlMessageSetChunkSize(msg *Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'set chunk size' invalid length <%d>", len(data))
	}
	c.readChunkSize = binary.BigEndian.Uint32(msg.Data.Bytes())
	if c.readChunkSize > MaxChunkSize {
		c.readChunkSize = MaxChunkSize
	}
	return
}
