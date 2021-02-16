package rtmp

import (
	"io"
)

type ServeConn struct {
	chunkHeader               ChunkHeader
	chunkSize                 int
	message                   []*Message // key:chunk stream id
	MaxChunkSize              int        // 防止chunk缓存过大
	acknowledgement           uint32     // acknowledgement消息的值
	windowAcknowledgementSize uint32     // window acknowledgement size的值
	bandWidth                 uint32     // set bandWidth消息的值
	bandWidthLimit            byte       // set bandWidth消息的值
}

func (s *ServeConn) getMessage(csid uint32) *Message {
	for _, m := range s.message {
		if m.CSID == csid {
			return m
		}
	}
	m := new(Message)
	m.CSID = csid
	s.message = append(s.message, m)
	return m
}

func (s *ServeConn) Serve(conn io.ReadWriter, handle func(*Message) error) (err error) {
	s.chunkSize = ChunkSize
	s.message = make([]*Message, 0)
	var n, i1, i2 int
	for {
		// 读取chunk header
		err = s.chunkHeader.Read(conn)
		if err != nil {
			return err
		}
		msg := s.getMessage(s.chunkHeader.CSID)
		// 消息头
		switch s.chunkHeader.FMT {
		case 0:
			msg.Timestamp = s.chunkHeader.MessageTimestamp
			msg.Length = s.chunkHeader.MessageLength
			msg.TypeID = s.chunkHeader.MessageTypeID
			msg.StreamID = s.chunkHeader.MessageStreamID
		case 1:
			if s.chunkHeader.MessageTimestamp == MaxMessageTimestamp {
				msg.Timestamp = s.chunkHeader.ExtendedTimestamp
			} else {
				msg.Timestamp += s.chunkHeader.MessageTimestamp
			}
			msg.Length = s.chunkHeader.MessageLength
			msg.TypeID = s.chunkHeader.MessageTypeID
		case 2:
			if s.chunkHeader.MessageTimestamp == MaxMessageTimestamp {
				msg.Timestamp = s.chunkHeader.ExtendedTimestamp
			} else {
				msg.Timestamp += s.chunkHeader.MessageTimestamp
			}
		default:
		}
		// 消息数据
		n = int(msg.Length) - len(msg.Data)
		if n > 0 {
			if n > s.chunkSize {
				n = s.chunkSize
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
			continue
		}
		// 处理消息
		err = handle(msg)
		if err != nil {
			return
		}
		msg.Length = 0
		msg.Data = msg.Data[:0]
	}
}

// func (s *ServeConn) handleMessage(msg *Message) error {

// 	switch s.chunkHeader.CSID {
// 	case ControlMessageChunkStreamID:
// 		switch s.chunkHeader.MessageTypeID {
// 		case ControlMessageSetChunkSize:
// 			err = s.handleControlMessageSetChunkSize()
// 		case ControlMessageAbort:
// 			err = s.handleControlMessageAbort()
// 		case ControlMessageAcknowledgement:
// 			err = s.handleControlMessageAcknowledgement()
// 		case ControlMessageWindowAcknowledgementSize:
// 			err = s.handleControlMessageWindowAcknowledgementSize()
// 		case ControlMessageSetBandWidth:
// 			err = s.handleControlMessageSetBandWidth()
// 		case UserControlMessage:
// 			err = s.handleUserControlMessage()
// 		default:
// 			return fmt.Errorf("unsupported cotrol message type <%d>", s.chunkHeader.MessageTypeID)
// 		}
// 		if err != nil {
// 			return
// 		}
// 	case CommandMessageChunkStreamID:
// 		switch s.chunkHeader.MessageTypeID {
// 		case CommandMessageAMF0:
// 			var amf interface{}
// 			amf, err = ReadAMF(s.conn)
// 			if err != nil {
// 				return
// 			}
// 			switch v := amf.(type) {
// 			case string:
// 				switch v {
// 				case "connect":
// 					err = s.handleCommandMessageConnect()
// 				case "call":
// 					err = s.handleCommandMessageCall()
// 				case "createStream":
// 					err = s.handleCommandMessageCreateStream()
// 				default:
// 					return fmt.Errorf("command message unsupported name <%s>", v)
// 				}
// 				if err != nil {
// 					return
// 				}
// 			default:
// 				return fmt.Errorf("command message name invalid amf type <%s>", reflect.TypeOf(amf).Kind().String())
// 			}
// 		default:
// 			return fmt.Errorf("unsupported command message type <%d>", s.chunkHeader.MessageTypeID)
// 		}
// 	}
// }

// func (s *ServeConn) handleCommandMessageCreateStream() (err error) {
// 	return
// }

// func (s *ServeConn) handleCommandMessageCall() (err error) {
// 	return
// }

// func (s *ServeConn) handleCommandMessageConnect() (err error) {
// 	// number
// 	_, err = ReadAMF(s.conn)
// 	if err != nil {
// 		return
// 	}
// 	return
// }

// func (s *ServeConn) handleUserControlMessage() (err error) {
// 	return
// }

// func (s *ServeConn) handleControlMessageSetBandWidth() (err error) {
// 	// chunk data
// 	_, err = io.ReadFull(s.conn, s.chunkData[:5])
// 	if err != nil {
// 		return
// 	}
// 	s.bandWidth = binary.BigEndian.Uint32(s.chunkData)
// 	s.bandWidthLimit = s.chunkData[4]
// 	return
// }

// func (s *ServeConn) handleControlMessageWindowAcknowledgementSize() (err error) {
// 	// chunk data
// 	_, err = io.ReadFull(s.conn, s.chunkData[:4])
// 	if err != nil {
// 		return
// 	}
// 	s.windowAcknowledgementSize = binary.BigEndian.Uint32(s.chunkData)
// 	return
// }

// func (s *ServeConn) handleControlMessageAcknowledgement() (err error) {
// 	// chunk data
// 	_, err = io.ReadFull(s.conn, s.chunkData[:4])
// 	if err != nil {
// 		return
// 	}
// 	s.acknowledgement = binary.BigEndian.Uint32(s.chunkData)
// 	return
// }

// func (s *ServeConn) handleControlMessageAbort() (err error) {
// 	// chunk data
// 	_, err = io.ReadFull(s.conn, s.chunkData[:4])
// 	if err != nil {
// 		return
// 	}
// 	csid := binary.BigEndian.Uint32(s.chunkData)
// 	// 移除消息
// 	for i := 0; i < len(s.message); i++ {
// 		if s.message[i] != nil && csid == s.chunkHeader.CSID {
// 			s.message[i].Length = 0
// 			break
// 		}
// 	}
// 	return
// }

// func (s *ServeConn) handleControlMessageSetChunkSize() (err error) {
// 	// chunk data
// 	_, err = io.ReadFull(s.conn, s.chunkData[:4])
// 	if err != nil {
// 		return
// 	}
// 	// chunk size
// 	size := int(binary.BigEndian.Uint32(s.chunkData))
// 	// 比设定的大
// 	if s.MaxChunkSize > 0 && size > int(s.MaxChunkSize) {
// 		return fmt.Errorf("control message set_chunk_size <%d> too big", size)
// 	}
// 	// 由于消息的最大长度为 16777215(0xFFFFFF)
// 	if size > 0xFFFFFF {
// 		size = 0xFFFFFF
// 	}
// 	if size > cap(s.chunkData) {
// 		s.chunkData = make([]byte, size)
// 	} else {
// 		s.chunkData = s.chunkData[:size]
// 	}
// 	return
// }
