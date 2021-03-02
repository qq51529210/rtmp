package rtmp

import (
	"bytes"
	"io"
	"sync"
)

const (
	ControlMessageSetChunkSize              = 1  // type id
	ControlMessageAbort                     = 2  // type id
	ControlMessageAcknowledgement           = 3  // type id
	ControlMessageWindowAcknowledgementSize = 5  // type id
	ControlMessageSetBandWidth              = 6  // type id
	CommandMessageAMF0                      = 20 // type id
	CommandMessageAMF3                      = 17 // type id
	DataMessageAMF0                         = 18 // type id
	DataMessageAMF3                         = 15 // type id
	SharedObjectMessageAMF0                 = 19 // type id
	SharedObjectMessageAMF3                 = 16 // type id
	AudioMessage                            = 8  // type id
	VideoMessage                            = 9  // type id
	AggregateMessage                        = 22 // type id
	UserControlMessage                      = 4  // type id

	UserControlMessageStreamBegin      = 0 // user control event
	UserControlMessageStreamEOF        = 1 // user control event
	UserControlMessageStreamDry        = 2 // user control event
	UserControlMessageSetBufferLength  = 3 // user control event
	UserControlMessageStreamIsRecorded = 4 // user control event
	UserControlMessagePingRequest      = 6 // user control event
	UserControlMessagePingResponse     = 7 // user control event

	ControlMessageStreamID      = 0 // stream id
	CommandMessageStreamID      = 0 // stream id
	ControlMessageChunkStreamID = 2 // chunk stream id
	CommandMessageChunkStreamID = 3 // chunk stream id
	VideoMessageChunkStreamID   = 4 // chunk stream id
	AudioMessageChunkStreamID   = 5 // chunk stream id
	DataMessageChunkStreamID    = 4 // chunk stream id
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

// 将data分成chunk发送，注意设置chunk.fmt
func WriteMessage(writer io.Writer, chunkHeader *ChunkHeader, chunkSize uint32, data []byte) error {
	// 第一个chunk
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
	chunkHeader.FMT = ChunkFmt3
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

type Message struct {
	TypeID    uint8        // 消息类型
	Timestamp uint32       // 时间戳
	StreamID  uint32       // 消息属于的流
	Length    uint32       // 消息的长度
	Data      bytes.Buffer // 消息的数据
}

func (m *Message) WriteB16(n uint16) {
	m.Data.WriteByte(byte(n >> 8))
	m.Data.WriteByte(byte(n))
}

func (m *Message) WriteB32(n uint32) {
	m.Data.WriteByte(byte(n >> 24))
	m.Data.WriteByte(byte(n >> 16))
	m.Data.WriteByte(byte(n >> 8))
	m.Data.WriteByte(byte(n))
}

func (m *Message) WriteAMFs(amfs ...interface{}) (err error) {
	for _, a := range amfs {
		err = WriteAMF(&m.Data, a)
		if err != nil {
			return
		}
	}
	return
}
