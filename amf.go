package rtmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"unsafe"
)

const (
	amfNumber     = 0
	amfBoolean    = 1
	amfString     = 2
	amfObject     = 3
	amfNull       = 5
	amfEcmaArray  = 8
	amfObjectEnd  = 9
	amfLongString = 12
)

var (
	errEmptyKey = errors.New("amf object empty key name")
)

// 从r中读取amf对象，返回对象数据或者错误
func ReadAMF(r io.Reader) (interface{}, error) {
	var buff [8]byte
	_, err := r.Read(buff[:1])
	if err != nil {
		return nil, err
	}
	switch buff[0] {
	case amfNumber:
		_, err = io.ReadFull(r, buff[:])
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(buff[:])), nil
	case amfBoolean:
		_, err := r.Read(buff[:1])
		if err != nil {
			return nil, err
		}
		return buff[0] != 0, nil
	case amfString:
		return readAMFString(r, buff[:])
	case amfObject:
		return readAMFObject(r, buff[:])
	case amfNull:
		return nil, nil
	case amfEcmaArray:
		return readAMFEcmaArray(r, buff[:])
	case amfLongString:
		return readAMFLongString(r, buff[:])
	default:
		return nil, fmt.Errorf("unsupported amf type <%d>", buff[0])
	}
}

func readAMFString(r io.Reader, b []byte) (string, error) {
	_, err := io.ReadFull(r, b[:2])
	if err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint16(b[:2])
	if n == 0 {
		return "", nil
	}
	str := make([]byte, n)
	_, err = io.ReadFull(r, str)
	if err != nil {
		return "", err
	}
	return *(*string)(unsafe.Pointer(&str)), nil
}

func readAMFLongString(r io.Reader, b []byte) (string, error) {
	_, err := io.ReadFull(r, b[:4])
	if err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint16(b[:4])
	if n == 0 {
		return "", nil
	}
	str := make([]byte, n)
	_, err = io.ReadFull(r, str)
	if err != nil {
		return "", err
	}
	return *(*string)(unsafe.Pointer(&str)), nil
}

func readAMFObject(r io.Reader, b []byte) (map[string]interface{}, error) {
	objects := make(map[string]interface{})
	var key string
	var value interface{}
	var err error
	var n uint16
	for {
		_, err = io.ReadFull(r, b[:3])
		if err != nil {
			return nil, err
		}
		// object end
		if b[0] == 0 && b[1] == 0 && b[2] == 9 {
			return objects, nil
		}
		// key
		n = binary.BigEndian.Uint16(b[:])
		if n == 0 {
			return nil, errEmptyKey
		}
		str := make([]byte, n)
		str[0] = b[2]
		_, err = io.ReadFull(r, str[1:])
		if err != nil {
			return nil, err
		}
		key = *(*string)(unsafe.Pointer(&str))
		// value
		value, err = ReadAMF(r)
		if err != nil {
			return nil, err
		}
		objects[key] = value
	}
}

func readAMFEcmaArray(r io.Reader, b []byte) (map[string]interface{}, error) {
	_, err := io.ReadFull(r, b[:4])
	if err != nil {
		return nil, err
	}
	objects := make(map[string]interface{})
	count := int(binary.BigEndian.Uint32(b[:]))
	if count == 0 {
		return objects, nil
	}
	var n int
	var key string
	var value interface{}
	for i := 0; i < count; i++ {
		_, err = io.ReadFull(r, b[:2])
		if err != nil {
			return nil, err
		}
		// key
		n = int(binary.BigEndian.Uint16(b[:]))
		if n == 0 {
			return nil, errEmptyKey
		}
		str := make([]byte, n)
		_, err = io.ReadFull(r, str)
		if err != nil {
			return nil, err
		}
		key = *(*string)(unsafe.Pointer(&str))
		value, err = ReadAMF(r)
		if err != nil {
			return nil, err
		}
		objects[key] = value
	}
	// object end
	_, err = io.ReadFull(r, b[:3])
	if err != nil {
		return nil, err
	}
	if b[0] != 0 && b[1] != 0 && b[2] != amfObjectEnd {
		return nil, fmt.Errorf("unsupported amf object end [0]:<%d>, [1]:<%d>, [2]:<%d>", b[0], b[1], b[2])
	}
	return objects, nil
}

func WriteAMFs(w io.Writer, a ...interface{}) (err error) {
	for _, v := range a {
		err = WriteAMF(w, v)
		if err != nil {
			return
		}
	}
	return
}

// 将数据格式成amf对象写入w
func WriteAMF(w io.Writer, amf interface{}) error {
	var buff [9]byte
	switch v := amf.(type) {
	case int:
		return writeAMFNumber(w, buff[:], float64(v))
	case uint:
		return writeAMFNumber(w, buff[:], float64(v))
	case int8:
		return writeAMFNumber(w, buff[:], float64(v))
	case uint8:
		return writeAMFNumber(w, buff[:], float64(v))
	case int16:
		return writeAMFNumber(w, buff[:], float64(v))
	case uint16:
		return writeAMFNumber(w, buff[:], float64(v))
	case int32:
		return writeAMFNumber(w, buff[:], float64(v))
	case uint32:
		return writeAMFNumber(w, buff[:], float64(v))
	case int64:
		return writeAMFNumber(w, buff[:], float64(v))
	case uint64:
		return writeAMFNumber(w, buff[:], float64(v))
	case float32:
		return writeAMFNumber(w, buff[:], float64(v))
	case float64:
		return writeAMFNumber(w, buff[:], v)
	case string:
		return writeAMFString(w, buff[:], v)
	case map[string]interface{}:
		return writeAMFObject(w, buff[:], v)
	case bool:
		return writeAMFBoolean(w, buff[:], v)
	case nil:
		buff[0] = amfNull
		_, err := w.Write(buff[:1])
		return err
	}
	panic(fmt.Errorf("unsupported data type <%s>", reflect.TypeOf(amf).Kind().String()))
}

func writeAMFNumber(w io.Writer, b []byte, n float64) error {
	b[0] = amfNumber
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(n))
	_, err := w.Write(b[:9])
	return err
}

func writeAMFString(w io.Writer, b []byte, s string) error {
	if len(s) > 0xffff {
		return writeAMFLongString(w, b, s)
	}
	b[0] = amfString
	binary.BigEndian.PutUint16(b[1:], uint16(len(s)))
	_, err := w.Write(b[:3])
	if err != nil {
		return err
	}
	if len(s) == 0 {
		return nil
	}
	_, err = w.Write(*(*[]byte)(unsafe.Pointer(&s)))
	return err
}

func writeAMFLongString(w io.Writer, b []byte, s string) error {
	b[0] = amfLongString
	binary.BigEndian.PutUint32(b[1:], uint32(len(s)))
	_, err := w.Write(b[:5])
	if err != nil {
		return err
	}
	if len(s) == 0 {
		return nil
	}
	_, err = w.Write(*(*[]byte)(unsafe.Pointer(&s)))
	return err
}

func writeAMFBoolean(w io.Writer, b []byte, o bool) error {
	b[0] = amfBoolean
	if o {
		b[1] = 1
	} else {
		b[1] = 0
	}
	_, err := w.Write(b[:2])
	return err
}

func writeAMFObject(w io.Writer, b []byte, o map[string]interface{}) (err error) {
	b[0] = amfObject
	_, err = w.Write(b[:1])
	if err != nil {
		return
	}
	for k, v := range o {
		if k == "" {
			continue
		}
		binary.BigEndian.PutUint16(b[:], uint16(len(k)))
		_, err = w.Write(b[:2])
		if err != nil {
			return err
		}
		_, err = w.Write(*(*[]byte)(unsafe.Pointer(&k)))
		if err != nil {
			return err
		}
		err = WriteAMF(w, v)
		if err != nil {
			return
		}
	}
	b[0] = 0
	b[1] = 0
	b[2] = amfObjectEnd
	_, err = w.Write(b[:3])
	return
}
