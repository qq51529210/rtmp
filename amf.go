package rtmp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	AMF_NUMBER = iota
	AMF_BOOLEAN
	AMF_STRING
	AMF_OBJECT
	AMF_MOVIECLIP /* RESERVED, NOT USED */
	AMF_NULL
	AMF_UNDEFINED
	AMF_REFERENCE
	AMF_ECMA_ARRAY
	AMF_OBJECT_END
	AMF_STRICT_ARRAY
	AMF_DATE
	AMF_LONG_STRING
	AMF_UNSUPPORTED
	AMF_RECORDSET /* RESERVED, NOT USED */
	AMF_XML_DOC
	AMF_TYPED_OBJECT
	AMF_AVMPLUS /* SWITCH TO AMF3 */
	AMF_INVALID = 0xFF
)

const (
	AMF3_UNDEFINED  = iota // 未定义;
	AMF3_NULL              // null;
	AMF3_FALSE             // false;
	AMF3_TRUE              // true;
	AMF3_INTEGER           // 数字int;
	AMF3_DOUBLE            // double;
	AMF3_STRING            // 字符串;
	AMF3_XML_DOC           // xml文档;
	AMF3_DATE              // 日期;
	AMF3_ARRAY             // 数组;
	AMF3_OBJECT            // 对象;
	AMF3_XML               // xml;
	AMF3_BYTE_ARRAY        // 字节数组;
)

const (
	AMF_STR_BUFFER_LEN      = 32
	AMF_LONG_STR_BUFFER_LEN = 256
)

// 从conn中读取amf对象，返回对象数据或者错误
func ReadAMF(conn io.Reader) (interface{}, error) {
	var buff [1]byte
	_, err := conn.Read(buff[:])
	if err != nil {
		return nil, err
	}
	switch buff[0] {
	case AMF_NUMBER:
		var b [8]byte
		_, err = io.ReadFull(conn, b[:])
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b[:])), nil
	case AMF_BOOLEAN:
		_, err := conn.Read(buff[:])
		if err != nil {
			return nil, err
		}
		return buff[0] != 0, nil
	case AMF_STRING:
		return readAMFString(conn)
	case AMF_OBJECT:
		return readAMFObject(conn)
	case AMF_NULL:
		return nil, nil
	case AMF_ECMA_ARRAY:
		return readAMFEcmaArray(conn)
	case AMF_LONG_STRING:
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
	var buff [AMF_STR_BUFFER_LEN]byte
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
	var buff [AMF_LONG_STR_BUFFER_LEN]byte
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
	var buff [AMF_STR_BUFFER_LEN]byte
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
	case map[string]interface{}:
	}
	return nil
}

func writeAMFNumber(conn io.Writer, n float64) error {
	var b [9]byte
	b[0] = AMF_NUMBER
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(n))
	_, err := conn.Write(b[:])
	return err
}

func writeAMFString(conn io.Writer, s string) error {
	if len(s) > 0xffff {
		return writeAMFLongString(conn, s)
	}
	var buff [AMF_STR_BUFFER_LEN]byte
	buff[0] = AMF_STRING
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
	var buff [AMF_LONG_STR_BUFFER_LEN]byte
	buff[0] = AMF_LONG_STRING
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
	b[0] = AMF_BOOLEAN
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
