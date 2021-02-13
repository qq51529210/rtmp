package rtmp

// const (
// 	MessageTypeSetChunkSize  = 1
// 	MessageTypeAbort         = 2
// 	MessageTypeACK           = 3
// 	MessageTypeControl       = 4
// 	MessageTypeWindowAckSize = 5
// 	MessageTypeSetBandWidth  = 6
// 	MessageTypeCmdAMF0       = 20
// 	MessageTypeDataAMF0      = 18
// 	MessageTypeAudio         = 8
// 	MessageTypeVideo         = 9
// )

// const (
// 	ControlMessageTypeStreamBegin      = 0
// 	ControlMessageTypeStreamEOF        = 1
// 	ControlMessageTypeStreamDry        = 2
// 	ControlMessageTypeSetBufferLength  = 3
// 	ControlMessageTypeStreamIsRecorded = 4
// 	ControlMessageTypePingRequest      = 6
// 	ControlMessageTypePingResponse     = 7
// )

// const (
// 	CSIDControl = 2
// 	CSIDCommand = 3
// )

// type AMFObject map[string]interface{}

// func WriteInt24(b []byte, n uint32) {
// 	b[0] = uint8(n >> 16)
// 	b[1] = uint8(n >> 8)
// 	b[2] = uint8(n)
// }

// func ReadInt24(b []byte) uint32 {
// 	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
// }

// func ReadAMF(b *bytes.Buffer) (interface{}, error) {
// 	t, e := b.ReadByte()
// 	if nil != e {
// 		return nil, e
// 	}
// 	switch t {
// 	case AMF_NUMBER:
// 		var m float64
// 		e = binary.Read(b, binary.BigEndian, &m)
// 		return m, e
// 	case AMF_BOOLEAN:
// 		o, e := b.ReadByte()
// 		return o != 0, e
// 	case AMF_STRING:
// 		o, e := readAMFString(b)
// 		return o, e
// 	case AMF_OBJECT:
// 		return readAMFObject(b)
// 	case AMF_ECMA_ARRAY:
// 		return readAMFECMAArray(b)
// 	case AMF_NULL:
// 	default:
// 		return nil, errors.New(fmt.Sprintf("invalid amf type %v", t))
// 	}
// 	return nil, nil
// }

// func readAMFECMAArray(b *bytes.Buffer) (AMFObject, error) {
// 	var m uint32
// 	e := binary.Read(b, binary.BigEndian, &m)
// 	if nil != e {
// 		return nil, e
// 	}
// 	o := make(AMFObject)
// 	for i := 0; i < int(m); i++ {
// 		// key
// 		k, e := readAMFString(b)
// 		if nil != e {
// 			return nil, e
// 		}
// 		if "" == k {
// 			// read amf_object_end
// 			_, e = b.ReadByte()
// 			return o, e
// 		}
// 		// value
// 		v, e := ReadAMF(b)
// 		if nil != e {
// 			return nil, e
// 		}
// 		o[k] = v
// 	}
// 	return o, nil
// }

// func readAMFString(b *bytes.Buffer) (string, error) {
// 	var sl uint16
// 	e := binary.Read(b, binary.BigEndian, &sl)
// 	if nil != e {
// 		return "", e
// 	}
// 	sb := make([]byte, sl)
// 	_, e = b.Read(sb)
// 	if nil != e {
// 		return "", e
// 	}
// 	return string(sb), e
// }

// func readAMFObject(b *bytes.Buffer) (AMFObject, error) {
// 	o := make(AMFObject)
// 	for {
// 		// key
// 		k, e := readAMFString(b)
// 		if nil != e {
// 			return nil, e
// 		}
// 		if "" == k {
// 			// read amf_object_end
// 			_, e = b.ReadByte()
// 			return o, e
// 		}
// 		// value
// 		v, e := ReadAMF(b)
// 		if nil != e {
// 			return nil, e
// 		}
// 		o[k] = v
// 	}
// }

// func WriteAMF(b *bytes.Buffer, v interface{}) error {
// 	if nil == v {
// 		b.WriteByte(AMF_NULL)
// 		return nil
// 	}
// 	switch v.(type) {
// 	case float64:
// 		b.WriteByte(AMF_NUMBER)
// 		binary.Write(b, binary.BigEndian, v)
// 	case bool:
// 		b.WriteByte(AMF_BOOLEAN)
// 		if v.(bool) {
// 			b.WriteByte(1)
// 		} else {
// 			b.WriteByte(0)
// 		}
// 	case string:
// 		b.WriteByte(AMF_STRING)
// 		s := v.(string)
// 		writeAMFString(b, &s)
// 	case AMFObject:
// 		b.WriteByte(AMF_OBJECT)
// 		writeAMFObject(b, v.(AMFObject))
// 		b.WriteByte(0)
// 		b.WriteByte(0)
// 		b.WriteByte(AMF_OBJECT_END)
// 	}
// 	return errors.New("invalid amf type")
// }

// func writeAMFString(b *bytes.Buffer, s *string) {
// 	binary.Write(b, binary.BigEndian, uint16(len(*s)))
// 	b.WriteString(*s)
// }

// func writeAMFObject(b *bytes.Buffer, o AMFObject) {
// 	for k, v := range o {
// 		writeAMFString(b, &k)
// 		WriteAMF(b, v)
// 	}
// }
