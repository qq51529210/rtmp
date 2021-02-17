package main

type Server struct {
}

// func (m *MessageReader) handleMessage() (err error) {
// 	// 处理一些属于不必要的的消息
// 	switch m.chunkHeader.MessageTypeID {
// 	case ControlMessageSetChunkSize:
// 		return m.handleControlMessageSetChunkSize()
// 	case ControlMessageAbort:
// 		return m.handleControlMessageAbort()
// 	case ControlMessageAcknowledgement:
// 		return m.handleControlMessageAcknowledgement()
// 	case ControlMessageWindowAcknowledgementSize:
// 		return m.handleControlMessageWindowAcknowledgementSize()
// 	case ControlMessageSetBandWidth:
// 		return m.handleControlMessageSetBandWidth()
// 		// case UserControlMessage:
// 		// return m.handleUserControlMessage()
// 		// case CommandMessageAMF0, CommandMessageAMF3:
// 		// return m.handleCommandMessage()
// 		// case DataMessageAMF0, DataMessageAMF3:
// 		// case SharedObjectMessageAMF0, SharedObjectMessageAMF3:
// 		// case AudioMessage:
// 		// case VideoMessage:
// 		// case AggregateMessage:
// 		// default:
// 		// return fmt.Errorf("unsupported message type <%d>", m.chunkHeader.MessageTypeID)
// 	}
// 	// 处理消息
// 	err = m.handle(msg)
// 	if err != nil {
// 		return
// 	}
// 	m.Length = 0
// 	m.Data = m.Data[:0]
// 	return
// }

// func (m *MessageReader) handleCommandMessage() (err error) {
// 	return
// }

// func (m *MessageReader) handleUserControlMessage() (err error) {
// 	return
// }

// func (m *MessageReader) handleControlMessageSetBandWidth() (err error) {
// 	if m.Length != 5 {
// 		return fmt.Errorf("control message set_bandwidth invalid length <%d>", m.Length)
// 	}
// 	m.bandWidth = binary.BigEndian.Uint32(m.Data)
// 	m.bandWidthLimit = m.Data[4]
// 	return
// }

// func (m *MessageReader) handleControlMessageWindowAcknowledgementSize() (err error) {
// 	if m.Length != 4 {
// 		return fmt.Errorf("control message window_acknowledgement_size invalid length <%d>", m.Length)
// 	}
// 	m.windowAcknowledgementSize = binary.BigEndian.Uint32(m.Data)
// 	return
// }

// func (m *MessageReader) handleControlMessageAcknowledgement() (err error) {
// 	if m.Length != 4 {
// 		return fmt.Errorf("control message acknowledgement invalid length <%d>", m.Length)
// 	}
// 	m.acknowledgement = binary.BigEndian.Uint32(m.Data)
// 	return
// }

// func (m *MessageReader) handleControlMessageAbort() (err error) {
// 	if m.Length != 4 {
// 		return fmt.Errorf("control message abort invalid length <%d>", m.Length)
// 	}
// 	// csid := binary.BigEndian.Uint32(m.Data)
// 	return
// }

// func (m *MessageReader) handleControlMessageSetChunkSize() (err error) {
// 	if m.Length != 4 {
// 		return fmt.Errorf("control message set_chunk_size invalid length <%d>", m.Length)
// 	}
// 	// chunk size
// 	size := int(binary.BigEndian.Uint32(m.Data))
// 	// 比设定的大
// 	if m.MaxChunkSize > 0 && size > int(m.MaxChunkSize) {
// 		return fmt.Errorf("control message set_chunk_size <%d> too big", size)
// 	}
// 	// 由于消息的最大长度为 16777215(0xFFFFFF)
// 	if size > 0xFFFFFF {
// 		size = 0xFFFFFF
// 	}
// 	m.chunkSize = size
// 	return
// }
