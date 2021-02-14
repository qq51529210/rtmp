package rtmp

type Message struct {
}

// import (
// 	"bytes"
// 	"encoding/binary"
// 	"fmt"
// 	"io"
// 	"errors"
// )

// /*
//  message

//  basic header
//  0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |fmt0|csid                        					   	 |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |fmt1|csid 	                         										 |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  0    1    2    3    4    5    6    7
//  +----+----+----+----+----+----+----+----+
//  |fmt2|csid                         	 |
//  +----+----+----+----+----+----+----+----+

//  message header
//  fmt 0
//  0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |timestamp           					   		   					   |message length                         |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |message length (3 bytes)     	   					   |message type id		           | msg stream id			   |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |message stream id (4 bytes)            										   |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+

//  fmt 1
//  0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |timestamp           					   		   					   |message length                               |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |message length (3 bytes)     	   					    |message type id		            |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+

//  fmt 2
//  0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7    0    1    2    3    4    5    6    7
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  |timestamp           					   		   					    |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+

// */

// func NewMessageReader(chunkSize uint32) *MessageReader {
// 	p := new(MessageReader)
// 	p.SetChunkSize(chunkSize)
// 	p.message = make(map[uint32]*Message)
// 	return p
// }

// type MessageReader struct {
// 	message                map[uint32]*Message //
// 	chunk                  []byte              //
// 	chunkSize              uint32              //
// 	lastChunkMessageSID    uint32              //
// 	lastChunkMessageLength uint32              //
// 	lastChunkMessageTypeID uint8               //
// }

// func (this *MessageReader) SetChunkSize(n uint32) {
// 	if n <= 0 {
// 		n = 128
// 	}
// 	this.chunkSize = n
// 	// max basic header + Message header + extended timestamp = 3B + 11B + 4B = 18B
// 	this.chunk = make([]byte, 18+n)
// }

// func (this *MessageReader) ReadMessageFrom(r io.Reader) (*Message, int, error) {
// 	n := 0
// 	for {
// 		m, n2, e := this.readMessageFrom(r)
// 		if nil != e {
// 			return nil, n, e
// 		}
// 		n += n2
// 		if m.Length == uint32(m.Payload.Len()) {
// 			return m, n, e
// 		}
// 	}
// }

// func (this *MessageReader) readMessageFrom(r io.Reader) (*Message, int, error) {
// 	n := 0
// 	// basic header
// 	n2, e := io.ReadFull(r, this.chunk[:1])
// 	if nil != e {
// 		return nil, n, e
// 	}
// 	n += n2
// 	_fmt := this.chunk[0] >> 6
// 	csid := uint32(this.chunk[0] & 0x3f)
// 	switch csid {
// 	case 0:
// 		// 2
// 		n2, e = io.ReadFull(r, this.chunk[:1])
// 		if nil != e {
// 			return nil, n, e
// 		}
// 		n += n2
// 		csid = uint32(this.chunk[0]) + 64
// 	case 1:
// 		// 3
// 		n2, e = io.ReadFull(r, this.chunk[:2])
// 		if nil != e {
// 			return nil, n, e
// 		}
// 		n += n2
// 		csid = uint32(this.chunk[1])*256 + uint32(this.chunk[0]) + 64
// 	}
// 	// message
// 	m := this.message[csid]
// 	if nil == m {
// 		m = NewMessage()
// 		this.message[csid] = m
// 	}
// 	switch _fmt {
// 	case 0x00:
// 		// 11
// 		n2, e = io.ReadFull(r, this.chunk[:11])
// 		if nil != e {
// 			return nil, n, e
// 		}
// 		n += n2
// 		// timestamp
// 		timestamp := ReadInt24(this.chunk[0:])
// 		// message length
// 		m.Length = ReadInt24(this.chunk[3:])
// 		m.Payload.Reset()
// 		// Message type id
// 		m.TypeID = this.chunk[6]
// 		// Message stream id
// 		m.StreamID = binary.BigEndian.Uint32(this.chunk[7:])
// 		// extended timestamp
// 		if timestamp >= 0xffffff {
// 			n2, e = io.ReadFull(r, this.chunk[:4])
// 			if nil != e {
// 				return nil, n, e
// 			}
// 			n += n2
// 			m.Timestamp = binary.BigEndian.Uint32(this.chunk[0:])
// 		} else {
// 			m.Timestamp = timestamp
// 		}
// 		this.lastChunkMessageLength = m.Length
// 		this.lastChunkMessageSID = m.StreamID
// 		this.lastChunkMessageTypeID = m.TypeID
// 	case 0x01:
// 		// 7
// 		n2, e = io.ReadFull(r, this.chunk[:7])
// 		if nil != e {
// 			return nil, n, e
// 		}
// 		n += n2
// 		// timestamp
// 		timestamp_delta := ReadInt24(this.chunk[0:])
// 		// message length
// 		m.Length = ReadInt24(this.chunk[3:])
// 		m.Payload.Reset()
// 		// Message type id
// 		m.TypeID = this.chunk[6]
// 		// extended timestamp
// 		if timestamp_delta >= 0xffffff {
// 			n2, e = io.ReadFull(r, this.chunk[:4])
// 			if nil != e {
// 				return nil, n, e
// 			}
// 			n += n2
// 			m.Timestamp = binary.BigEndian.Uint32(this.chunk[0:])
// 		} else {
// 			m.Timestamp += timestamp_delta
// 		}
// 		this.lastChunkMessageLength = m.Length
// 		this.lastChunkMessageTypeID = m.TypeID
// 	case 0x02:
// 		// 3
// 		n2, e = io.ReadFull(r, this.chunk[:3])
// 		if nil != e {
// 			return nil, n, e
// 		}
// 		n += n2
// 		// timestamp delta
// 		timestamp_delta := ReadInt24(this.chunk[0:])
// 		// extended timestamp
// 		if timestamp_delta >= 0xffffff {
// 			n2, e = io.ReadFull(r, this.chunk[:4])
// 			if nil != e {
// 				return nil, n, e
// 			}
// 			n += n2
// 			m.Timestamp = binary.BigEndian.Uint32(this.chunk[0:])
// 		} else {
// 			m.Timestamp += timestamp_delta
// 		}
// 	case 0x03:
// 		// 0
// 	}
// 	// read chunk size
// 	tn := m.Length - uint32(m.Payload.Len())
// 	if tn > this.chunkSize {
// 		_, e = io.CopyN(m.Payload, r, int64(this.chunkSize))
// 	} else if tn > 0 {
// 		_, e = io.CopyN(m.Payload, r, int64(tn))
// 	}
// 	return m, n, e
// }

// func NewMessageWriter(chunkSize uint32) *MessageWriter {
// 	p := new(MessageWriter)
// 	p.SetChunkSize(chunkSize)
// 	return p
// }

// type MessageWriter struct {
// 	chunk     []byte // chunk buffer, header and data
// 	chunkSize uint32 // chunk real size
// }

// func (this *MessageWriter) SetChunkSize(n uint32) {
// 	if n <= 0 {
// 		n = 128
// 	}
// 	this.chunkSize = n
// 	// max basic header + message header + extended timestamp = 3B + 11B + 4B = 18B
// 	this.chunk = make([]byte, 18)
// }

// func (this *MessageWriter) WriteMessageTo(w io.Writer, m *Message, _fmt uint8, csid uint32) (int, error) {
// 	// basic header
// 	n := this.writeBasicHeader(_fmt, csid)
// 	switch _fmt {
// 	case 0x00:
// 		// timestamp
// 		if m.Timestamp >= 0xffffff {
// 			WriteInt24(this.chunk[n:], 1)
// 		} else {
// 			WriteInt24(this.chunk[n:], m.Timestamp)
// 		}
// 		n += 3
// 		// message length
// 		WriteInt24(this.chunk[n:], m.Length)
// 		n += 3
// 		// message type id
// 		this.chunk[n] = m.TypeID
// 		n++
// 		// message stream id
// 		binary.BigEndian.PutUint32(this.chunk[n:], m.StreamID)
// 		n += 4
// 		if m.Timestamp >= 0xffffff {
// 			binary.BigEndian.PutUint32(this.chunk[n:], m.Timestamp-0xffffff)
// 			n += 4
// 		}
// 	case 0x01:
// 		// timestamp
// 		if m.Timestamp >= 0xffffff {
// 			WriteInt24(this.chunk[n:], 1)
// 		} else {
// 			WriteInt24(this.chunk[n:], m.Timestamp)
// 		}
// 		n += 3
// 		// message length
// 		WriteInt24(this.chunk[n:], m.Length)
// 		n += 3
// 		// message type id
// 		this.chunk[n] = m.TypeID
// 		n++
// 		if m.Timestamp >= 0xffffff {
// 			binary.BigEndian.PutUint32(this.chunk[n:], m.Timestamp-0xffffff)
// 			n += 4
// 		}
// 	case 0x02:
// 		// timestamp
// 		if m.Timestamp >= 0xffffff {
// 			WriteInt24(this.chunk[n:], 1)
// 		} else {
// 			WriteInt24(this.chunk[n:], m.Timestamp)
// 		}
// 		n += 3
// 		if m.Timestamp >= 0xffffff {
// 			binary.BigEndian.PutUint32(this.chunk[n:], m.Timestamp-0xffffff)
// 			n += 4
// 		}
// 	case 0x03:
// 	default:
// 		return n, errors.New(fmt.Sprintf("invalid basic header fmt %d", _fmt))
// 	}
// 	//fmt.Println(this.chunk[:n])
// 	_, e := w.Write(this.chunk[:n])
// 	if nil != e {
// 		return n, e
// 	}
// 	if m.Payload.Len() < 1 {
// 		return n, nil
// 	}
// 	// chunk data
// 	if uint32(m.Payload.Len()) <= this.chunkSize {
// 		n2, e := w.Write(m.Payload.Bytes())
// 		return n + n2, e
// 	}
// 	d := m.Payload.Bytes()
// 	n2, e := w.Write(d[:this.chunkSize])
// 	if nil != e {
// 		return n + n2, e
// 	}
// 	d = d[this.chunkSize:]
// 	// fmt 3
// 	for {
// 		// basic header
// 		n2 = this.writeBasicHeader(0x03, csid)
// 		n2, e = w.Write(this.chunk[:n2])
// 		if nil != e {
// 			return n, e
// 		}
// 		n += n2
// 		// chunk data
// 		if uint32(len(d)) <= this.chunkSize {
// 			n2, e := w.Write(d)
// 			return n + n2, e
// 		} else {
// 			n2, e := w.Write(d[:this.chunkSize])
// 			if nil != e {
// 				return n + n2, e
// 			}
// 			n += n2
// 			d = d[this.chunkSize:]
// 		}
// 	}
// 	return n, nil
// }

// func (this *MessageWriter) writeBasicHeader(_fmt uint8, csid uint32) int {
// 	this.chunk[0] = _fmt << 6
// 	switch {
// 	case csid < 64:
// 		this.chunk[0] |= uint8(csid)
// 		return 1
// 	case 64 <= csid && csid < 320:
// 		this.chunk[1] = uint8(csid - 64)
// 		return 2
// 	default:
// 		this.chunk[0] |= 0x3F
// 		binary.LittleEndian.PutUint16(this.chunk[1:], uint16(csid-64))
// 		return 3
// 	}
// }

// type Message struct {
// 	TypeID    uint8  // 1B
// 	Length    uint32 // 3B
// 	Timestamp uint32 // 4B
// 	StreamID  uint32 // 3B
// 	Payload   *bytes.Buffer
// }

// func NewMessage() *Message {
// 	m := new(Message)
// 	m.Payload = bytes.NewBuffer(nil)
// 	return m
// }

// func (this *Message) String() string {
// 	return fmt.Sprintf("Message -> TypeID:%d, Length:%d, Timestamp:%d, StreamID:%d, Payload:%d", this.TypeID, this.Length, this.Timestamp, this.StreamID, this.Payload.Len())
// }

// func (this *Message) InitSetChunkSize(size uint32) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeSetChunkSize
// 	this.Timestamp = 0
// 	this.StreamID = 0
// 	binary.Write(this.Payload, binary.BigEndian, size)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitAbort(csid uint32) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeAbort
// 	this.Timestamp = 0
// 	this.StreamID = 0
// 	binary.Write(this.Payload, binary.BigEndian, csid)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitAcknowledge(sn uint32) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeACK
// 	this.Timestamp = 0
// 	this.StreamID = 0
// 	binary.Write(this.Payload, binary.BigEndian, sn)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitWindowAcknowledgeSize(size uint32) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeWindowAckSize
// 	this.Timestamp = 0
// 	this.StreamID = 0
// 	binary.Write(this.Payload, binary.BigEndian, size)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitSetBandwidth(size uint32, limit uint8) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeSetBandWidth
// 	this.Timestamp = 0
// 	this.StreamID = 0
// 	binary.Write(this.Payload, binary.BigEndian, size)
// 	this.Payload.WriteByte(limit)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitStreamBegin(id uint32) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeControl
// 	this.Timestamp = 0
// 	binary.Write(this.Payload, binary.BigEndian, uint16(ControlMessageTypeStreamBegin))
// 	binary.Write(this.Payload, binary.BigEndian, id)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitSetBufferLength(id, ms uint32) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeControl
// 	this.Timestamp = 0
// 	binary.Write(this.Payload, binary.BigEndian, uint16(ControlMessageTypeSetBufferLength))
// 	binary.Write(this.Payload, binary.BigEndian, id)
// 	binary.Write(this.Payload, binary.BigEndian, ms)
// 	this.Length = uint32(this.Payload.Len())
// }

// func (this *Message) InitAMF0(name string, tid float64, cmd, info interface{}) {
// 	this.Payload.Reset()
// 	this.TypeID = MessageTypeCmdAMF0
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
