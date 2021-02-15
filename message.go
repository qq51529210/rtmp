package rtmp

import (
	"encoding/binary"
	"sync"
)

// 协议控制消息1-6，使用sid=0和csid=2，接收到必须立即生效。
const (
	ProtocolControlMessageSetChunkSize  = 1
	ProtocolControlMessageAbort         = 2
	ProtocolControlMessageAck           = 3
	ProtocolControlMessageWindowAckSize = 5
	ProtocolControlMessageSetBandWidth  = 6

	MessageTypeIDControl = 4

	ProtocolControlMessageSID  = 0 // 协议控制消息1-6，stream id
	ProtocolControlMessageCSID = 2 // 协议控制消息1-6，stream id
	ProtocolControlMessageFMT  = 3

	CommandMessageCSID = 3

	MessageTypeIDCmdAMF0  = 20
	MessageTypeIDDataAMF0 = 18
	MessageTypeIDAudio    = 8
	MessageTypeIDVideo    = 9

	ControlMessageTypeIDStreamBegin      = 0
	ControlMessageTypeIDStreamEOF        = 1
	ControlMessageTypeIDStreamDry        = 2
	ControlMessageTypeIDSetBufferLength  = 3
	ControlMessageTypeIDStreamIsRecorded = 4
	ControlMessageTypeIDPingRequest      = 6
	ControlMessageTypeIDPingResponse     = 7
)

var (
	messagePool sync.Pool
)

func init() {
	messagePool.New = func() interface{} {
		m := new(Message)
		return m
	}
}

type Message struct {
	Timestamp uint32
	TypeID    uint32
	StreamID  uint32
	Length    uint32
	Data      []byte
	FMT       byte
	CSID      uint32
}

func (m *Message) resizeBuffer(n int) {
	if cap(m.Data) >= n {
		m.Data = m.Data[:n]
	} else {
		m.Data = make([]byte, n)
	}
}

func (m *Message) initProtocolControlMessage() {
	m.FMT = ProtocolControlMessageFMT
	m.CSID = ProtocolControlMessageCSID
	m.StreamID = ProtocolControlMessageSID
}

// 最大块大小设置的话最少为128字节
func (m *Message) ProtocolControlMessageSetChunkSize(size uint32) {
	if size < ChunkSize {
		size = ChunkSize
	}
	m.initProtocolControlMessage()
	m.TypeID = ProtocolControlMessageSetChunkSize
	m.resizeBuffer(4)
	binary.BigEndian.PutUint32(m.Data, size)
}

// 获取数据
func (m *Message) GetProtocolControlMessageSetChunkSize() uint32 {
	return binary.BigEndian.Uint32(m.Data)
}

// 终止正在接收csid的message，消息会被丢弃。
func (m *Message) ProtocolControlMessageAbort(csid uint32) {
	m.initProtocolControlMessage()
	m.TypeID = ProtocolControlMessageAbort
	m.resizeBuffer(4)
	binary.BigEndian.PutUint32(m.Data, csid)
}

// 获取数据
func (m *Message) GetProtocolControlMessageAbort() uint32 {
	return binary.BigEndian.Uint32(m.Data)
}

// 接收到等同于窗口大小的字节之后必须要发送给对端一个确认，
// 窗口大小是指发送者在没有收到接收者确认之前发送的最大数量的字节。
// sequenceNumber也就是目前接收到的字节数。
func (m *Message) ProtocolControlMessageAck(sequenceNumber uint32) {
	m.initProtocolControlMessage()
	m.TypeID = ProtocolControlMessageAck
	m.resizeBuffer(4)
	binary.BigEndian.PutUint32(m.Data, sequenceNumber)
}

// 获取数据
func (m *Message) GetProtocolControlMessageAck() uint32 {
	return binary.BigEndian.Uint32(m.Data)
}

// 通知对端的ack的窗口大小，size:窗口大小。接收端收到这条消息（或者会话建立之后），必须响应一个ProtocolControlMessageAck。
func (m *Message) ProtocolControlMessageWindowAckSize(size uint32) {
	m.initProtocolControlMessage()
	m.TypeID = ProtocolControlMessageWindowAckSize
	m.resizeBuffer(4)
	binary.BigEndian.PutUint32(m.Data, size)
}

// 获取数据
func (m *Message) GetProtocolControlMessageWindowAckSize() uint32 {
	return binary.BigEndian.Uint32(m.Data)
}

// 限制其对端的输出带宽。接收端收到这条消息，应该响应一个ProtocolControlMessageWindowAckSize。
// limitType:0 - Hard：对端应该限制其输出带宽到指示的窗口大小。
// 1 - Soft：对端应该限制其输出带宽到知识的窗口大小，或者已经有限制在其作用的话就取两者之间的较小值。
// 2 - Dynamic：如果先前的限制类型为 Hard，处理这个消息就好像它被标记为 Hard，否则的话忽略这个消息
func (m *Message) ProtocolControlMessageSetBandWidth(bandwidth uint32, limitType uint8) {
	m.initProtocolControlMessage()
	m.TypeID = ProtocolControlMessageSetBandWidth
	m.resizeBuffer(5)
	binary.BigEndian.PutUint32(m.Data, bandwidth)
	m.Data[4] = limitType
}

// 获取数据
func (m *Message) GetProtocolControlMessageSetBandWidth() (uint32, uint8) {
	return binary.BigEndian.Uint32(m.Data)
}

// func (this *Message) InitAMF0(name string, tid float64, cmd, info interface{}) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeIDCmdAMF0
// 	this.Timestamp = 0
// 	WriteAMF(this.Payload, name)
// 	WriteAMF(this.Payload, tid)
// 	WriteAMF(this.Payload, cmd)
// 	WriteAMF(this.Payload, info)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitMediaHeader(tid uint8, ts uint32, b *bytes.Buffer) {
// 	this.Payload.Reset()
// 	this.TypeID = tid
// 	this.Timestamp = ts
// 	this.StreamID = 0
// 	this.Payload.Write(b.Bytes())
// 	this.Length = uint32(this.Payload.Len())
// }
