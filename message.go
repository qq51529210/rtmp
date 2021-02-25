package rtmp

import (
	"bytes"
	"fmt"
	"io"
	"sync"
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

var (
	msgPool sync.Pool
)

func init() {
	msgPool.New = func() interface{} {
		return new(Message)
	}
}

// 获取缓存
func GetMessage() *Message {
	msg := msgPool.Get().(*Message)
	msg.Data.Reset()
	return msg
}

// 回收缓存
func PutMessage(msg *Message) {
	msgPool.Put(msg)
}

func UserControlMessageString(event uint16) string {
	switch event {
	case UserControlMessageStreamBegin:
		return "user control message stream begin"
	case UserControlMessageStreamEOF:
		return "user control message stream eof"
	case UserControlMessageStreamDry:
		return "user control message stream dry"
	case UserControlMessageSetBufferLength:
		return "user control message set buffer length"
	case UserControlMessageStreamIsRecorded:
		return "user control message stream is recorded"
	case UserControlMessagePingRequest:
		return "user control message ping request"
	case UserControlMessagePingResponse:
		return "user control message ping response"
	default:
		return fmt.Sprintf("user control message event <%d>", event)
	}
}

type Message struct {
	TypeID    uint8        // 消息类型
	Timestamp uint32       // 时间戳
	StreamID  uint32       // 消息属于的流
	Length    uint32       // 消息的长度
	Data      bytes.Buffer // 消息的数据
}

func (m *Message) WriteBigEndianUint16(n uint16) {
	m.Data.WriteByte(byte(n >> 8))
	m.Data.WriteByte(byte(n))
}

func (m *Message) WriteBigEndianUint32(n uint32) {
	m.Data.WriteByte(byte(n >> 24))
	m.Data.WriteByte(byte(n >> 16))
	m.Data.WriteByte(byte(n >> 8))
	m.Data.WriteByte(byte(n))
}

func WriteMessage(writer io.Writer, chunkHeader *ChunkHeader, chunkSize uint32, data []byte) error {
	// 第一个chunk
	chunkHeader.FMT = 0
	err := chunkHeader.Write(writer)
	if err != nil {
		return err
	}
	n := uint32(len(data))
	if n > chunkSize {
		n = chunkSize
	}
	_, err = writer.Write(data[:n])
	if err != nil {
		return err
	}
	data = data[n:]
	// 其他的chunk
	chunkHeader.FMT = 3
	for len(data) > 0 {
		err = chunkHeader.Write(writer)
		if err != nil {
			return err
		}
		n = uint32(len(data))
		if n > chunkSize {
			n = chunkSize
		}
		_, err = writer.Write(data[:n])
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
