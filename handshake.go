package rtmp

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/rand"
	"sync"
	"time"
)

const (
	rtmpProtoVersion = 3
)

var (
	fmsKey = []byte{
		0x47, 0x65, 0x6e, 0x75, 0x69, 0x6e, 0x65, 0x20,
		0x41, 0x64, 0x6f, 0x62, 0x65, 0x20, 0x46, 0x6c,
		0x61, 0x73, 0x68, 0x20, 0x4d, 0x65, 0x64, 0x69,
		0x61, 0x20, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
		0x20, 0x30, 0x30, 0x31, // Genuine Adobe Flash Media Server 001
		0xf0, 0xee, 0xc2, 0x4a, 0x80, 0x68, 0xbe, 0xe8,
		0x2e, 0x00, 0xd0, 0xd1, 0x02, 0x9e, 0x7e, 0x57,
		0x6e, 0xec, 0x5d, 0x2d, 0x29, 0x80, 0x6f, 0xab,
		0x93, 0xb8, 0xe6, 0x36, 0xcf, 0xeb, 0x31, 0xae,
	}
	fpKey = []byte{
		0x47, 0x65, 0x6E, 0x75, 0x69, 0x6E, 0x65, 0x20,
		0x41, 0x64, 0x6F, 0x62, 0x65, 0x20, 0x46, 0x6C,
		0x61, 0x73, 0x68, 0x20, 0x50, 0x6C, 0x61, 0x79,
		0x65, 0x72, 0x20, 0x30, 0x30, 0x31, // Genuine Adobe Flash Player 001
		0xF0, 0xEE, 0xC2, 0x4A, 0x80, 0x68, 0xBE, 0xE8,
		0x2E, 0x00, 0xD0, 0xD1, 0x02, 0x9E, 0x7E, 0x57,
		0x6E, 0xEC, 0x5D, 0x2D, 0x29, 0x80, 0x6F, 0xAB,
		0x93, 0xB8, 0xE6, 0x36, 0xCF, 0xEB, 0x31, 0xAE,
	}
	mathRand     = rand.New(rand.NewSource(time.Now().UnixNano()))
	errDigestC1  = errors.New("c1 digest error")
	errDigestC2  = errors.New("c2 digest error")
	errDigestS1  = errors.New("s1 digest error")
	errDigestS2  = errors.New("s2 digest error")
	fpKeyPool    sync.Pool
	fpKey30Pool  sync.Pool
	fmsKeyPool   sync.Pool
	fmsKey36Pool sync.Pool
	// Version      = 1381256528
)

func init() {
	fpKeyPool.New = func() interface{} {
		return hmac.New(sha256.New, fpKey)
	}
	fmsKeyPool.New = func() interface{} {
		return hmac.New(sha256.New, fmsKey)
	}
	fpKey30Pool.New = func() interface{} {
		return hmac.New(sha256.New, fpKey[:30])
	}
	fmsKey36Pool.New = func() interface{} {
		return hmac.New(sha256.New, fmsKey[:36])
	}
}

type (
	handshakeDigest [32]byte
	handshakeKey    [128]byte
	handshakeRandom [1528]byte
	handshakeBuffer [1537]byte
)

func handshakeSchema0DigestOffset(buff []byte) int {
	// 764-32-4=728
	return 13 + ((int(buff[9]) + int(buff[10]) + int(buff[11]) + int(buff[12])) % 728)
}

func handshakeSchema1DigestOffset(buff []byte) int {
	// 1537-764=773
	// 764-32-4=728
	return 777 + ((int(buff[773]) + int(buff[774]) + int(buff[775]) + int(buff[776])) % 728)
}

func handshakeGenSchema0Digest(buff, digest []byte, hashPool *sync.Pool) {
	offset := handshakeSchema0DigestOffset(buff)
	h := hashPool.Get().(hash.Hash)
	h.Reset()
	h.Write(buff[1:offset])
	h.Write(buff[offset+32:])
	h.Sum(digest[:0])
	hashPool.Put(h)
	copy(buff[offset:], digest)
}

func handshakeGenSchema1Digest(buff, digest []byte, hashPool *sync.Pool) {
	offset := handshakeSchema1DigestOffset(buff)
	h := hashPool.Get().(hash.Hash)
	h.Reset()
	h.Write(buff[1:offset])
	h.Write(buff[offset+32:])
	h.Sum(digest[:0])
	hashPool.Put(h)
	copy(buff[offset:], digest)
}

func handshakeCheckDigest(buff, digest []byte, hashPool *sync.Pool) (schema int) {
	h := hashPool.Get().(hash.Hash)
	var sum handshakeDigest
	// schema 0
	n1 := handshakeSchema0DigestOffset(buff)
	n2 := n1 + 32
	h.Reset()
	h.Write(buff[1:n1])
	h.Write(buff[n2:])
	h.Sum(sum[:0])
	if bytes.Compare(buff[n1:n2], sum[:]) == 0 {
		hashPool.Put(h)
		copy(digest[:], sum[:])
		return
	}
	// schema 1
	n1 = handshakeSchema1DigestOffset(buff)
	n2 = n1 + 32
	h.Reset()
	h.Write(buff[1:n1])
	h.Write(buff[n2:])
	h.Sum(sum[:0])
	if bytes.Compare(buff[n1:n2], sum[:]) == 0 {
		hashPool.Put(h)
		copy(digest[:], sum[:])
		schema = 1
		return
	}
	hashPool.Put(h)
	schema = -1
	return
}

func handshakeGenDigest2(buff, data []byte, hashPool *sync.Pool) {
	var digest handshakeDigest
	h := hashPool.Get().(hash.Hash)
	h.Reset()
	h.Write(data[:])
	h.Sum(digest[:0])
	hashPool.Put(h)
	// 1537-32=1505
	h = hmac.New(sha256.New, digest[:])
	h.Write(buff[1:1505])
	h.Sum(digest[:0])
	copy(buff[1505:], digest[:])
	return
}

func handshakeCheckDigest2(buff, data []byte, hashPool *sync.Pool) bool {
	h := hashPool.Get().(hash.Hash)
	var digest handshakeDigest
	h.Reset()
	h.Write(data[:])
	h.Sum(digest[:0])
	hashPool.Put(h)
	// 1537-32=1505
	h = hmac.New(sha256.New, digest[:])
	h.Write(buff[1:1505])
	h.Sum(digest[:0])
	return bytes.Compare(digest[:], buff[1505:]) == 0
}

// conn一般是net.Conn。
// version是自定义的版本（不是RTMP_PROTO_VERSION哦）。
// 返回客户端c1消息的自定义版本，或者错误。
func HandshakeAccept(conn io.ReadWriter, version uint32) (uint32, error) {
	var handshakeBuffer handshakeBuffer
	buff := handshakeBuffer[:]
	// c0 c1
	_, err := io.ReadFull(conn, buff)
	if err != nil {
		return 0, err
	}
	if buff[0] != rtmpProtoVersion {
		return 0, fmt.Errorf("invalid protocol version <%d>", buff[0])
	}
	c1Version := binary.BigEndian.Uint32(buff[5:])
	if c1Version != 0 {
		// complex handshake
		return c1Version, handshakeComplexAccept(conn, buff, version)
	}
	c1Time := binary.BigEndian.Uint32(buff[1:])
	var c1Random handshakeRandom
	copy(c1Random[:], buff[9:])
	// s1
	s1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[1:], s1Time)
	binary.BigEndian.PutUint32(buff[5:], version)
	mathRand.Read(buff[9:])
	var s1Random handshakeRandom
	copy(s1Random[:], buff[9:])
	_, err = conn.Write(buff)
	if err != nil {
		return 0, err
	}
	// s2
	binary.BigEndian.PutUint32(buff[1:], c1Time)
	binary.BigEndian.PutUint32(buff[5:], uint32(time.Now().Unix()))
	copy(buff[9:], c1Random[:])
	_, err = conn.Write(buff[1:])
	if err != nil {
		return 0, err
	}
	// c2
	_, err = io.ReadFull(conn, buff[1:])
	if err != nil {
		return 0, err
	}
	c2Time := binary.BigEndian.Uint32(buff[1:])
	if c2Time != s1Time {
		return 0, fmt.Errorf("c2 time <%d> no equal s1 time <%d>", c2Time, s1Time)
	}
	c2Random := buff[9:]
	for i := 0; i < len(c2Random); i++ {
		if c2Random[i] != s1Random[i] {
			return 0, fmt.Errorf("c2 random <%d> no equal s1 random <%d> at <%d>", c2Random[i], s1Random[i], i)
		}
	}
	return c1Version, nil
}

func handshakeComplexAccept(conn io.ReadWriter, buff []byte, version uint32) error {
	// c1Time := binary.BigEndian.Uint32(buff[1:])
	var c1Digest handshakeDigest
	schema := handshakeCheckDigest(buff, c1Digest[:], &fpKey30Pool)
	if schema < 0 {
		return errDigestC1
	}
	// s0s1
	s1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[1:], s1Time)
	binary.BigEndian.PutUint32(buff[5:], version)
	mathRand.Read(buff[9:])
	var s1Digest handshakeDigest
	if schema == 0 {
		handshakeGenSchema0Digest(buff, s1Digest[:], &fmsKey36Pool)
	} else {
		handshakeGenSchema1Digest(buff, s1Digest[:], &fmsKey36Pool)
	}
	_, err := conn.Write(buff)
	if err != nil {
		return err
	}
	// s2
	// binary.BigEndian.PutUint32(buff[1:], c1Time)
	// binary.BigEndian.PutUint32(buff[5:], uint32(time.Now().Unix()))
	mathRand.Read(buff[1:1505])
	handshakeGenDigest2(buff, c1Digest[:], &fmsKeyPool)
	_, err = conn.Write(buff[1:])
	if err != nil {
		return err
	}
	// c2
	_, err = io.ReadFull(conn, buff[1:])
	if err != nil {
		return err
	}
	if !handshakeCheckDigest2(buff, s1Digest[:], &fpKeyPool) {
		return errDigestC2
	}
	// c2Time := binary.BigEndian.Uint32(buff[1:])
	// if c2Time != s1Time {
	// 	return fmt.Errorf("c2 time <%d> no equal s1 time <%d>", c2Time, s1Time)
	// }
	return nil
}

// conn一般是net.Conn。
// version是自定义的版本，0表示简单握手，非0则是复杂握手。
// 返回服务端s1消息的自定义版本，或者错误。
func HandshakeDial(conn io.ReadWriter, version uint32) (uint32, error) {
	var handshakeBuffer handshakeBuffer
	buff := handshakeBuffer[:]
	// c0c1
	buff[0] = rtmpProtoVersion
	c1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[1:], c1Time)
	binary.BigEndian.PutUint32(buff[5:], version)
	mathRand.Read(buff[9:])
	var c1Random handshakeRandom
	copy(c1Random[:], buff[9:])
	if version != 0 {
		// complex
		return handshakeComplexDial(conn, buff, c1Time, version)
	}
	_, err := conn.Write(buff[:])
	if err != nil {
		return 0, err
	}
	// s0s1
	_, err = io.ReadFull(conn, buff[:])
	if err != nil {
		return 0, err
	}
	if buff[0] != rtmpProtoVersion {
		return 0, fmt.Errorf("invalid protocol version <%d>", buff[0])
	}
	s1Version := binary.BigEndian.Uint32(buff[5:])
	// c2
	binary.BigEndian.PutUint32(buff[5:], uint32(time.Now().Unix()))
	_, err = conn.Write(buff[1:])
	if err != nil {
		return 0, err
	}
	// s2
	_, err = io.ReadFull(conn, buff[1:])
	if err != nil {
		return 0, err
	}
	s2Time1 := binary.BigEndian.Uint32(buff[1:])
	if s2Time1 != c1Time {
		return 0, fmt.Errorf("s2 time <%d> no equal c1 time <%d>", s2Time1, c1Time)
	}
	s2Random := buff[9:]
	for i := 0; i < len(s2Random); i++ {
		if c1Random[i] != s2Random[i] {
			return 0, fmt.Errorf("s2 random <%d> no equal c1 random <%d> at <%d>", s2Random[i], c1Random[i], i)
		}
	}
	return s1Version, nil
}

// digest - key的模式
func handshakeComplexDial(conn io.ReadWriter, buff []byte, c1Time, version uint32) (uint32, error) {
	var c1Digest handshakeDigest
	handshakeGenSchema0Digest(buff, c1Digest[:], &fpKey30Pool)
	_, err := conn.Write(buff[:])
	if err != nil {
		return 0, err
	}
	// s0 s1
	_, err = io.ReadFull(conn, buff[:])
	if err != nil {
		return 0, err
	}
	if buff[0] != rtmpProtoVersion {
		return 0, fmt.Errorf("invalid protocol version <%d>", buff[0])
	}
	var s1Digest handshakeDigest
	schema := handshakeCheckDigest(buff, s1Digest[:], &fmsKey36Pool)
	if schema < 0 {
		return 0, errDigestS1
	}
	// s1Time := binary.BigEndian.Uint32(buff[1:])
	s1Version := binary.BigEndian.Uint32(buff[5:])
	// c2
	// binary.BigEndian.PutUint32(buff[1:], s1Time)
	// binary.BigEndian.PutUint32(buff[5:], uint32(time.Now().Unix()))
	mathRand.Read(buff[1:1505])
	handshakeGenDigest2(buff, s1Digest[:], &fpKeyPool)
	_, err = conn.Write(buff[1:])
	if err != nil {
		return 0, err
	}
	// s2
	_, err = io.ReadFull(conn, buff[1:])
	if err != nil {
		return 0, err
	}
	if !handshakeCheckDigest2(buff, c1Digest[:], &fmsKeyPool) {
		return 0, errDigestS2
	}
	// s2Time1 := binary.BigEndian.Uint32(buff[1:])
	// if s2Time1 != c1Time {
	// 	return 0, fmt.Errorf("s2 time <%d> no equal c1 time <%d>", s2Time1, c1Time)
	// }
	return s1Version, nil
}
