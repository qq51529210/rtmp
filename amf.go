package rtmp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
)

const (
	amfNumber     = 0
	amfBoolean    = 1
	amfString     = 2
	amfObject     = 3
	amfNull       = 5
	amfEcmaArray  = 8
	amfLongString = 12
)

const (
	amfStrBufferLen     = 32
	amfLongStrBufferLen = 256
)

// 从conn中读取amf对象，返回对象数据或者错误
func ReadAMF(conn io.Reader) (interface{}, error) {
	var buff [1]byte
	_, err := conn.Read(buff[:])
	if err != nil {
		return nil, err
	}
	switch buff[0] {
	case amfNumber:
		var b [8]byte
		_, err = io.ReadFull(conn, b[:])
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b[:])), nil
	case amfBoolean:
		_, err := conn.Read(buff[:])
		if err != nil {
			return nil, err
		}
		return buff[0] != 0, nil
	case amfString:
		return readAMFString(conn)
	case amfObject:
		return readAMFObject(conn)
	case amfNull:
		return nil, nil
	case amfEcmaArray:
		return readAMFEcmaArray(conn)
	case amfLongString:
		return readAMFLongString(conn)
	default:
		return nil, fmt.Errorf("unsupported amf type <%d>", buff[0])
	}
}

func readAMFString(c io.Reader) (string, error) {
	var b [2]byte
	_, err := io.ReadFull(c, b[:])
	if err != nil {
		return "", err
	}
	length := int(binary.BigEndian.Uint16(b[:]))
	var n, m int
	var buff [amfStrBufferLen]byte
	var str strings.Builder
	for {
		n, err = c.Read(buff[:m])
		if err != nil {
			return "", err
		}
		str.Write(buff[:n])
		length -= n
		if length <= 0 {
			break
		}
		if length > len(buff) {
			m = len(buff)
		} else {
			m = length
		}
	}
	return str.String(), nil
}

func readAMFLongString(c io.Reader) (string, error) {
	var b [4]byte
	_, err := io.ReadFull(c, b[:])
	if err != nil {
		return "", err
	}
	length := int(binary.BigEndian.Uint32(b[:]))
	var n, m int
	var buff [amfLongStrBufferLen]byte
	var str strings.Builder
	for {
		n, err = c.Read(buff[:m])
		if err != nil {
			return "", err
		}
		str.Write(buff[:n])
		length -= n
		if length <= 0 {
			break
		}
		if length > len(buff) {
			m = len(buff)
		} else {
			m = length
		}
	}
	return str.String(), nil
}

func readAMFObject(c io.Reader) (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	var k string
	var v interface{}
	var err error
	var b [3]byte
	// first key length
	var length int
	var n, m int
	var buff [amfStrBufferLen]byte
	var str strings.Builder
	for {
		_, err = io.ReadFull(c, b[:])
		if err != nil {
			return nil, err
		}
		// object end
		if b[0] == 0 && b[1] == 0 && b[2] == 9 {
			break
		}
		// key
		length = int(binary.BigEndian.Uint16(b[:]))
		str.WriteByte(b[2])
		length--
		for {
			n, err = c.Read(buff[:m])
			if err != nil {
				return nil, err
			}
			str.Write(buff[:n])
			length -= n
			if length <= 0 {
				break
			}
			if length > len(buff) {
				m = len(buff)
			} else {
				m = length
			}
		}
		k = str.String()
		str.Reset()
		// value
		v, err = ReadAMF(c)
		if err != nil {
			return nil, err
		}
		obj[k] = v
	}
	return obj, nil
}

func readAMFEcmaArray(c io.Reader) (map[string]interface{}, error) {
	var b [4]byte
	_, err := io.ReadFull(c, b[:])
	if err != nil {
		return nil, err
	}
	obj := make(map[string]interface{})
	var k string
	var v interface{}
	for i := 0; i < int(binary.BigEndian.Uint32(b[:])); i++ {
		k, err = readAMFString(c)
		if err != nil {
			return nil, err
		}
		v, err = ReadAMF(c)
		if err != nil {
			return nil, err
		}
		obj[k] = v
	}
	return obj, nil
}

// 将数据格式成amf对象写入conn
func WriteAMF(conn io.Writer, amf interface{}) error {
	switch v := amf.(type) {
	case int:
		return writeAMFNumber(conn, float64(v))
	case uint:
		return writeAMFNumber(conn, float64(v))
	case int8:
		return writeAMFNumber(conn, float64(v))
	case uint8:
		return writeAMFNumber(conn, float64(v))
	case int16:
		return writeAMFNumber(conn, float64(v))
	case uint16:
		return writeAMFNumber(conn, float64(v))
	case int32:
		return writeAMFNumber(conn, float64(v))
	case uint32:
		return writeAMFNumber(conn, float64(v))
	case int64:
		return writeAMFNumber(conn, float64(v))
	case uint64:
		return writeAMFNumber(conn, float64(v))
	case float32:
		return writeAMFNumber(conn, float64(v))
	case float64:
		return writeAMFNumber(conn, v)
	case string:
		return writeAMFString(conn, v)
	case map[string]interface{}:
		return writeAMFObject(conn, v)
	case nil:
		return writeAMFNil(conn)
	}
	panic(fmt.Errorf("unsupported data type <%s>", reflect.TypeOf(amf).Kind().String()))
}

func writeAMFNumber(conn io.Writer, n float64) error {
	var b [9]byte
	b[0] = amfNumber
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(n))
	_, err := conn.Write(b[:])
	return err
}

func writeAMFString(conn io.Writer, s string) error {
	if len(s) > 0xffff {
		return writeAMFLongString(conn, s)
	}
	var buff [amfStrBufferLen]byte
	buff[0] = amfString
	binary.BigEndian.PutUint16(buff[1:], uint16(len(s)))
	n := copy(buff[3:], s)
	_, err := conn.Write(buff[:n])
	if err != nil {
		return err
	}
	s = s[n:]
	for len(s) > 0 {
		n = copy(buff[:], s)
		_, err := conn.Write(buff[:n])
		if err != nil {
			return err
		}
		s = s[n:]
	}
	return nil
}

func writeAMFLongString(conn io.Writer, s string) error {
	var buff [amfLongStrBufferLen]byte
	buff[0] = amfLongString
	binary.BigEndian.PutUint32(buff[1:], uint32(len(s)))
	n := copy(buff[5:], s)
	_, err := conn.Write(buff[:n])
	if err != nil {
		return err
	}
	s = s[n:]
	for len(s) > 0 {
		n = copy(buff[:], s)
		_, err := conn.Write(buff[:n])
		if err != nil {
			return err
		}
		s = s[n:]
	}
	return nil
}

func writeAMFBoolean(conn io.Writer, boolean bool) error {
	var b [2]byte
	b[0] = amfBoolean
	if boolean {
		b[1] = 1
	}
	_, err := conn.Write(b[:])
	return err
}

func writeAMFObject(conn io.Writer, obj map[string]interface{}) (err error) {
	for k, v := range obj {
		err = writeAMFString(conn, k)
		if err != nil {
			return
		}
		err = WriteAMF(conn, v)
		if err != nil {
			return
		}
	}
	var b [3]byte
	b[2] = AMF_OBJECT_END
	_, err = conn.Write(b[:])
	return
}

func writeAMFNil(conn io.Writer) (err error) {
	var b [1]byte
	b[0] = amfNull
	_, err = conn.Write(b[:])
	return err
}
