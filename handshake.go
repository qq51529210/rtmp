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
	RTMP_PROTO_VERSION          = 3
	handshakeBufferLen          = 1537
	handshakeRandomLen          = handshakeBufferLen - 9
	handshakeKeyLen             = 128
	handshakeDigestLen          = 32
	handshakeProtocolVersion    = 0
	handshakeData               = handshakeProtocolVersion + 1
	handshakeTime1              = handshakeData
	handshakeTime2              = handshakeTime1 + 4
	handshakeVersion            = handshakeTime1 + 4
	handshakeRandom             = handshakeTime1 + 8
	handshakeSchema0            = handshakeTime1 + 8
	handshakeSchema0DigestBlock = handshakeSchema0
	handshakeSchema0KeyBlock    = handshakeSchema0DigestBlock + 764
	handshakeSchema0Key         = handshakeSchema0KeyBlock + 764 - 4
	handshakeSchema1            = handshakeTime1 + 8
	handshakeSchema1KeyBlock    = handshakeSchema1
	handshakeSchema1Key         = handshakeSchema1KeyBlock + 764 - 4
	handshakeSchema1DigestBlock = handshakeSchema1KeyBlock + 764

	handshakeC2S2Digest = handshakeBufferLen - handshakeDigestLen

	handshakeKeyBlockRandomLen    = 764 - handshakeKeyLen - 4
	handshakeDigestBlockRandomLen = 764 - handshakeDigestLen - 4
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
	Version      = 1381256528
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

// 求哈希
func genHash(key, data []byte, digest [handshakeDigestLen]byte) {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	h.Sum(digest[:0])
}

// 4个字节相加，然后取余数，offset表示key block的偏移
func handshakeKeyOffset(buff []byte, offset int) int {
	return offset + (int(buff[0])+int(buff[1])+int(buff[2])+int(buff[3]))%handshakeKeyBlockRandomLen
}

// 4个字节相加，然后取余数，length表示digest block的偏移
func handshakeDigestOffset(buff []byte, offset int) int {
	return offset + 4 + (int(buff[0])+int(buff[1])+int(buff[2])+int(buff[3]))%handshakeDigestBlockRandomLen
}

// 生成buff的digest。
// buff是数据。digest用于接收生成的哈希值。hashPool是hash.Hash的缓存池。offset是digest block的偏移。
func handshakeGenDigest(buff []byte, digest [handshakeDigestLen]byte, hashPool *sync.Pool, offset int) {
	// 计算出数据块内的偏移+4+数据块整体的偏移 = digest在buff中的偏移
	offset = handshakeDigestOffset(buff[offset:], offset)
	// 哈希
	h := hashPool.Get().(hash.Hash)
	h.Reset()
	h.Write(buff[handshakeData:offset])
	h.Write(buff[offset+handshakeDigestLen:])
	h.Sum(digest[:0])
	hashPool.Put(h)
	// 拷贝到buff中
	copy(buff[offset:], digest[:])
}

// 检查数据，返回-1表示检查不通过，否则返回digest数据块在buff中的偏移
func handshakeCheckDigest(buff []byte, hashPool *sync.Pool) (int, int) {
	h := hashPool.Get().(hash.Hash)
	var sum [handshakeDigestLen]byte
	// schema 0
	offset := handshakeDigestOffset(buff[handshakeSchema0DigestBlock:], handshakeDigestBlockRandomLen)
	h.Reset()
	h.Write(buff[handshakeData:offset])
	h.Write(buff[offset+handshakeDigestLen:])
	h.Sum(sum[:0])
	// 检查
	digest := buff[offset:]
	find := true
	for i := 0; i < len(sum); i++ {
		if sum[i] != digest[i] {
			find = false
			break
		}
	}
	if find {
		hashPool.Put(h)
		return offset, handshakeKeyOffset(buff[handshakeSchema0Key:], handshakeSchema0KeyBlock)
	}
	// schema 1
	offset = handshakeDigestOffset(buff[handshakeSchema1DigestBlock:], handshakeSchema1DigestBlock)
	h.Reset()
	h.Write(buff[handshakeData:offset])
	h.Write(buff[offset+handshakeDigestLen:])
	h.Sum(sum[:0])
	digest = buff[offset:]
	find = true
	for i := 0; i < len(sum); i++ {
		if sum[i] != digest[i] {
			find = false
			break
		}
	}
	if find {
		hashPool.Put(h)
		return offset, handshakeKeyOffset(buff[handshakeSchema1Key:], handshakeSchema1KeyBlock)
	}
	hashPool.Put(h)
	return -1, -1
}

// conn一般是net.COnn。
// version是自定义的版本（不是RTMP_PROTO_VERSION哦）。
// 返回客户端c1消息的自定义版本，或者错误。
func HandshakeAccept(conn io.ReadWriter, version uint32) (uint32, error) {
	var buff [handshakeBufferLen]byte
	// read c0 c1
	_, err := io.ReadFull(conn, buff[:])
	if err != nil {
		return 0, err
	}
	// c0 protocol version
	if buff[handshakeProtocolVersion] != RTMP_PROTO_VERSION {
		return 0, fmt.Errorf("invalid protocol version <%d>", buff[handshakeProtocolVersion])
	}
	// c1 version
	c1Version := binary.BigEndian.Uint32(buff[handshakeVersion:])
	if c1Version != 0 {
		// complex handshake
		return c1Version, handshakeComplexAccept(conn, buff[:], version)
	}
	// c1 time
	c1Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// c1 random保存，待会在s2 random发送
	var c1Random [handshakeRandomLen]byte
	copy(c1Random[:], buff[handshakeRandom:])
	// s1 time
	s1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[handshakeTime1:], s1Time)
	// s1 version
	binary.BigEndian.PutUint32(buff[handshakeVersion:], version)
	// s1 random保存，待会与c2 random比较
	var s1Random [handshakeRandomLen]byte
	mathRand.Read(s1Random[:])
	copy(buff[handshakeRandom:], s1Random[:])
	// write s0 s1
	_, err = conn.Write(buff[:])
	if err != nil {
		return 0, err
	}
	// read c2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return 0, err
	}
	// c2 time1 = s1 time
	c2Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	if c2Time != s1Time {
		return 0, fmt.Errorf("c2 time <%d> no equal s1 time <%d>", c2Time, s1Time)
	}
	// c2 time2
	// binary.BigEndian.Uint32(buff[handshakeTime2:])
	// c2 random = s1 random
	c2Random := buff[handshakeRandom:]
	for i := 0; i < handshakeRandomLen; i++ {
		if c2Random[i] != s1Random[i] {
			return 0, fmt.Errorf("c2 random <%d> no equal s1 random <%d> at <%d>", c2Random[i], s1Random[i], i)
		}
	}
	// s2 time1 = c1 time1
	binary.BigEndian.PutUint32(buff[handshakeTime1:], c1Time)
	// s2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// s2 random = c1 random
	copy(buff[handshakeRandom:], c1Random[:])
	// write s2
	_, err = conn.Write(buff[handshakeData:])
	return c1Version, nil
}

func handshakeComplexAccept(conn io.ReadWriter, buff []byte, version uint32) error {
	// c1 digest
	digestOffset, _ := handshakeCheckDigest(buff, &fpKey30Pool)
	if digestOffset < 0 {
		return errDigestC1
	}
	// c1 time
	c1Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// c1 key，拷贝到s1 key
	// var c1Key [handshakeKeyLen]byte
	// copy(c1Key[:], buff[keyOffset:])
	// c1 digest
	var c1Digest [handshakeDigestLen]byte
	copy(c1Digest[:], buff[digestOffset:])
	// s1 time
	s1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[handshakeTime1:], s1Time)
	// s1 version
	binary.BigEndian.PutUint32(buff[handshakeVersion:], version)
	// s1 random，就用c1 random
	// mathRand.Read(buff[handshakeRandom:])
	// s1 key
	// copy(buff[keyOffset:], c1Key[:])
	// s1 digest
	var s1Digest [handshakeDigestLen]byte
	handshakeGenDigest(buff, s1Digest, &fmsKey36Pool, digestOffset)
	// write s0 s1
	_, err := conn.Write(buff)
	if err != nil {
		return err
	}
	// read c2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return err
	}
	// c2 digest
	c2Digest := buff[handshakeC2S2Digest:]
	var digest [handshakeDigestLen]byte
	genHash(s1Digest[:], buff[handshakeData:handshakeC2S2Digest], digest)
	if bytes.Compare(c2Digest, digest[:]) != 0 {
		return errDigestC2
	}
	// c2 time1 = s1 time
	c2Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// c2 time1
	if c2Time != s1Time {
		return fmt.Errorf("c2 time <%d> no equal s1 time <%d>", c2Time, s1Time)
	}
	// c2 time2
	// c2Time2 := binary.BigEndian.Uint32(buff[handshakeTime2:])
	// s2 time1 = c1 time1
	binary.BigEndian.PutUint32(buff[handshakeTime1:], c1Time)
	// s2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// s2 random
	mathRand.Read(buff[handshakeRandom:handshakeC2S2Digest])
	// s2 digest
	genHash(c1Digest[:], buff[handshakeData:handshakeC2S2Digest], digest)
	copy(buff[handshakeC2S2Digest:], digest[:])
	// write s2
	_, err = conn.Write(buff[handshakeData:])
	return nil
}

// conn一般是net.COnn。
// version是自定义的版本，0表示简单握手，非0则是复杂握手。
// 返回服务端s1消息的自定义版本，或者错误。
func HandshakeDial(conn io.ReadWriter, version uint32) (uint32, error) {
	if version != 0 {
		// complex
		return handshakeComplexDial(conn, version)
	}
	var buff [handshakeBufferLen]byte
	// c0 protocal version
	buff[handshakeProtocolVersion] = RTMP_PROTO_VERSION
	// c1 time
	c1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[handshakeTime1:], c1Time)
	// c1 version
	binary.BigEndian.PutUint32(buff[handshakeVersion:], version)
	// c1 random
	mathRand.Read(buff[handshakeRandom:])
	var c1Random [handshakeRandomLen]byte
	copy(c1Random[:], buff[handshakeRandom:])
	// write c0 c1
	_, err := conn.Write(buff[:])
	if err != nil {
		return 0, err
	}
	// read s0 s1
	_, err = io.ReadFull(conn, buff[:])
	if err != nil {
		return 0, err
	}
	// s0 protocol version
	if buff[handshakeProtocolVersion] != RTMP_PROTO_VERSION {
		return 0, fmt.Errorf("invalid protocol version <%d>", buff[handshakeProtocolVersion])
	}
	// s1 time，在c2 time1中发送
	// s1Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// s1 version
	s1Version := binary.BigEndian.Uint32(buff[handshakeVersion:])
	// c2 time1
	// binary.BigEndian.PutUint32(buff[handshakeTime2:], s1Time)
	// c2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// c2 random，就是s1 random
	// write c2
	_, err = conn.Write(buff[handshakeData:])
	if err != nil {
		return 0, err
	}
	// read s2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return 0, err
	}
	// s2 time1
	s2Time1 := binary.BigEndian.Uint32(buff[handshakeTime1:])
	if s2Time1 != c1Time {
		return 0, fmt.Errorf("s2 time <%d> no equal c1 time <%d>", s2Time1, c1Time)
	}
	// s2 time2
	// s2Time2 := binary.BigEndian.Uint32(buff[handshakeTime2:])
	// s2 random
	for i := 0; i < len(buff[handshakeRandom:]); i++ {
		if c1Random[i] != buff[i] {
			return 0, fmt.Errorf("s2 random <%d> no equal c1 random <%d> at <%d>", buff[i], c1Random[i], i)
		}
	}
	return s1Version, nil
}

// digest - key的模式
func handshakeComplexDial(conn io.ReadWriter, version uint32) (uint32, error) {
	var buff [handshakeBufferLen]byte
	// c0 protocal version
	buff[handshakeProtocolVersion] = RTMP_PROTO_VERSION
	// c1 time
	c1Time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[handshakeTime1:], c1Time)
	// c1 version
	binary.BigEndian.PutUint32(buff[handshakeVersion:], version)
	// c1 random
	mathRand.Read(buff[handshakeRandom:])
	// c1 digest
	var c1Digest [handshakeDigestLen]byte
	handshakeGenDigest(buff[:], c1Digest, &fpKey30Pool, handshakeSchema0DigestBlock)
	// read s0 s1
	_, err := io.ReadFull(conn, buff[:])
	if err != nil {
		return 0, err
	}
	// s1 digest
	digestOffset, _ := handshakeCheckDigest(buff[:], &fmsKey36Pool)
	if digestOffset < 0 {
		return 0, errDigestS1
	}
	// s1 time
	s1Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// s1 version
	s1Version := binary.BigEndian.Uint32(buff[handshakeVersion:])
	// s1 digest
	var s1Digest [handshakeDigestLen]byte
	copy(s1Digest[:], buff[digestOffset:])
	// c2 tim1，s1 time1
	binary.BigEndian.PutUint32(buff[handshakeTime1:], s1Time)
	// c2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// c2 random
	mathRand.Read(buff[handshakeRandom:])
	// c2 digest
	var digest [handshakeDigestLen]byte
	genHash(s1Digest[:], buff[handshakeData:handshakeC2S2Digest], digest)
	copy(buff[handshakeC2S2Digest:], digest[:])
	// write c2
	_, err = conn.Write(buff[handshakeData:])
	if err != nil {
		return 0, err
	}
	// read s2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return 0, err
	}
	// s2 digest
	s2Digest := buff[handshakeC2S2Digest:]
	// 以c1 digest为key的hash
	genHash(c1Digest[:], buff[handshakeData:handshakeC2S2Digest], digest)
	// 比较
	if bytes.Compare(digest[:], s2Digest) != 0 {
		return 0, errDigestS2
	}
	// s2 time1
	s2Time1 := binary.BigEndian.Uint32(buff[handshakeTime1:])
	if s2Time1 != c1Time {
		return 0, fmt.Errorf("s2 time <%d> no equal c1 time <%d>", s2Time1, c1Time)
	}
	// s2 time2
	// s2Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	return s1Version, nil
}
