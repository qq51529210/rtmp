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
	RTMP_PROTO_VERSION = 3
	// CLIENT_VERSION     = 0x80000702
	// SERVER_VERSION     = 0x04050001
)

const (
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

	handshakeKeyOffsetRemainder    = 764 - handshakeKeyLen - 4
	handshakeDigestOffsetRemainder = 764 - handshakeDigestLen - 4
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

type Handshake struct {
	RemoteProtocolVersion byte   // c0/s0版本号
	RemoteVersion         uint32 // c1/s1版本号
	RemoteTime1           uint32 // c1/s1消息time
	RemoteTime2           uint32 // c2/s2消息time2
}

func (h *Handshake) encCS1(buff []byte, version uint32) uint32 {
	// protocol version
	buff[handshakeProtocolVersion] = RTMP_PROTO_VERSION
	// time
	_time := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(buff[handshakeTime1:], _time)
	// version
	binary.BigEndian.PutUint32(buff[handshakeVersion:], version)
	// random
	mathRand.Read(buff[handshakeRandom:])
	return _time
}

func (h *Handshake) encC2(buff []byte) {
	// time1 = remote time1
	binary.BigEndian.PutUint32(buff[handshakeTime1:], h.RemoteTime1)
	// time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// random
	mathRand.Read(buff[handshakeRandom:])
}

func (h *Handshake) decCS1(buff []byte) {
	// protocol version
	h.RemoteProtocolVersion = buff[handshakeProtocolVersion]
	// time
	h.RemoteTime1 = binary.BigEndian.Uint32(buff[handshakeTime1:])
	// version
	h.RemoteVersion = binary.BigEndian.Uint32(buff[handshakeVersion:])
}

// client handshake
func (h *Handshake) Dial(conn io.ReadWriter, c1Version uint32) (err error) {
	if c1Version != 0 {
		// complex
		return h.complexDial(conn, c1Version)
	}
	var buff [handshakeBufferLen]byte
	// c0 c1
	c1Time := h.encCS1(buff[:], c1Version)
	// c1 random = s2 random
	var c1Random [handshakeRandomLen]byte
	copy(c1Random[:], buff[handshakeRandom:])
	// write c0 c1
	_, err = conn.Write(buff[:])
	if err != nil {
		return
	}
	// read s0 s1
	_, err = io.ReadFull(conn, buff[:])
	if err != nil {
		return
	}
	h.decCS1(buff[:])
	// c2 time1 = s1 time
	binary.BigEndian.PutUint32(buff[handshakeTime1:], h.RemoteTime1)
	// c2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// write c2，c2 random = s1 random
	_, err = conn.Write(buff[handshakeData:])
	if err != nil {
		return
	}
	// read s2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return
	}
	// s2 time1
	s2Time1 := binary.BigEndian.Uint32(buff[handshakeTime1:])
	if s2Time1 != c1Time {
		return fmt.Errorf("s2 time <%d> no equal c1 time <%d>", s2Time1, c1Time)
	}
	// s2 time2
	h.RemoteTime2 = binary.BigEndian.Uint32(buff[handshakeTime2:])
	// s2 random
	for i := 0; i < len(buff[handshakeRandom:]); i++ {
		if c1Random[i] != buff[i] {
			return fmt.Errorf("s2 random <%d> no equal c1 random <%d> at <%d>", buff[i], c1Random[i], i)
		}
	}
	return
}

// client complex handshake
func (h *Handshake) complexDial(conn io.ReadWriter, c1Version uint32) (err error) {
	// digest - key的模式
	var buff [handshakeBufferLen]byte
	// c0 c1
	c1Time := h.encCS1(buff[:], c1Version)
	// c1 digest
	hh := fpKey30Pool.Get().(hash.Hash)
	c1Digest := h.genDigest(buff[:], hh, handshakeSchema0DigestBlock)
	fpKey30Pool.Put(hh)
	// read s0 s1
	_, err = io.ReadFull(conn, buff[:])
	if err != nil {
		return
	}
	// s1 digest
	hh = fmsKey36Pool.Get().(hash.Hash)
	digestOffset := h.checkDigest(buff[:], hh)
	fmsKey36Pool.Put(hh)
	if digestOffset < 0 {
		return errDigestS1
	}
	h.decCS1(buff[:])
	// 以s1 digest为key的hash
	hh = hmac.New(sha256.New, buff[digestOffset:digestOffset+32])
	// c2
	h.encC2(buff[:])
	// c2 digest
	c2Digest := h.hash(hh, buff[handshakeData:handshakeC2S2Digest])
	copy(buff[handshakeC2S2Digest:], c2Digest)
	// write c2
	_, err = conn.Write(buff[handshakeData:])
	if err != nil {
		return
	}
	// read s2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return
	}
	// s2 digest
	s2Digest := buff[handshakeC2S2Digest:]
	// 以c1 digest为key的hash
	hh = hmac.New(sha256.New, c1Digest)
	hh.Write(buff[handshakeData:handshakeC2S2Digest])
	digest := hh.Sum(nil)
	// 比较
	if bytes.Compare(digest, s2Digest) != 0 {
		return errDigestS2
	}
	// s2 time1
	s2Time1 := binary.BigEndian.Uint32(buff[handshakeTime1:])
	if s2Time1 != c1Time {
		return fmt.Errorf("s2 time <%d> no equal c1 time <%d>", s2Time1, c1Time)
	}
	// s2 time2
	h.RemoteTime2 = binary.BigEndian.Uint32(buff[handshakeTime1:])
	return
}

// server handshake
func (h *Handshake) Accept(conn io.ReadWriter, s1Version uint32) (err error) {
	var buff [handshakeBufferLen]byte
	// read c0 c1
	_, err = io.ReadFull(conn, buff[:])
	if err != nil {
		return
	}
	h.decCS1(buff[:])
	if h.RemoteVersion != 0 {
		return h.complexAccept(conn, buff[:], s1Version)
	}
	// c1 random = s2 random
	var c1Random [handshakeRandomLen]byte
	copy(c1Random[:], buff[handshakeRandom:])
	// s1 time = c2 time
	s1Time := h.encCS1(buff[:], s1Version)
	// s1 random
	var s1Random [handshakeRandomLen]byte
	copy(s1Random[:], buff[handshakeRandom:])
	// write s0 s1
	_, err = conn.Write(buff[:])
	if err != nil {
		return
	}
	// read c2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return
	}
	// c2 time1 = s1 time
	c2Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// c2 time1
	if c2Time != s1Time {
		return fmt.Errorf("c2 time <%d> no equal s1 time <%d>", c2Time, s1Time)
	}
	// c2 time2
	h.RemoteTime2 = binary.BigEndian.Uint32(buff[handshakeTime2:])
	// c2 random = s1 random
	c2Random := buff[handshakeRandom:]
	for i := 0; i < handshakeRandomLen; i++ {
		if c2Random[i] != s1Random[i] {
			return fmt.Errorf("c2 random <%d> no equal s1 random <%d> at <%d>", c2Random[i], s1Random[i], i)
		}
	}
	// s2 time1 = c1 time1
	binary.BigEndian.PutUint32(buff[handshakeTime1:], h.RemoteTime1)
	// s2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// s2 random
	copy(buff[handshakeRandom:], c1Random[:])
	// write s2
	_, err = conn.Write(buff[handshakeData:])
	return
}

func (h *Handshake) complexAccept(conn io.ReadWriter, buff []byte, s1Version uint32) (err error) {
	// c1 digest
	hh := fpKey30Pool.Get().(hash.Hash)
	digestOffset := h.checkDigest(buff, hh)
	fpKey30Pool.Put(hh)
	if digestOffset < 0 {
		return errDigestC1
	}
	// 以c1 digest为key的hash
	c1DigestHash := hmac.New(sha256.New, buff[digestOffset:digestOffset+32])
	// s0 s1
	s1Time := h.encCS1(buff[:], s1Version)
	// s1 digest
	hh = fmsKey36Pool.Get().(hash.Hash)
	s1Digest := h.genDigest(buff, hh, digestOffset)
	fmsKey36Pool.Put(hh)
	// write s0 s1
	_, err = conn.Write(buff)
	if err != nil {
		return
	}
	// read c2
	_, err = io.ReadFull(conn, buff[handshakeData:])
	if err != nil {
		return
	}
	// c2 digest
	c2Digest := buff[handshakeC2S2Digest:]
	digest := h.hash(hmac.New(sha256.New, s1Digest), buff[handshakeData:handshakeC2S2Digest])
	if bytes.Compare(c2Digest, digest) != 0 {
		return errDigestC2
	}
	// c2 time1 = s1 time
	c2Time := binary.BigEndian.Uint32(buff[handshakeTime1:])
	// c2 time1
	if c2Time != s1Time {
		return fmt.Errorf("c2 time <%d> no equal s1 time <%d>", c2Time, s1Time)
	}
	// c2 time2
	h.RemoteTime2 = binary.BigEndian.Uint32(buff[handshakeTime2:])
	// s2 time1 = c1 time1
	binary.BigEndian.PutUint32(buff[handshakeTime1:], h.RemoteTime1)
	// s2 time2
	binary.BigEndian.PutUint32(buff[handshakeTime2:], uint32(time.Now().Unix()))
	// s2 random
	mathRand.Read(buff[handshakeRandom:handshakeC2S2Digest])
	// s2 digest
	digest = h.hash(c1DigestHash, buff[handshakeData:handshakeC2S2Digest])
	copy(buff[handshakeC2S2Digest:], digest)
	// write s2
	_, err = conn.Write(buff[handshakeData:])
	return
}

// 求哈希
func (h *Handshake) hash(hh hash.Hash, data []byte) []byte {
	hh.Reset()
	hh.Write(data)
	return hh.Sum(nil)
}

// 4个字节相加，然后取余数
func (h *Handshake) keyOffset(b []byte) int {
	return (int(b[0]) + int(b[1]) + int(b[2]) + int(b[3])) % handshakeKeyOffsetRemainder
}

// 4个字节相加，然后取余数
func (h *Handshake) digestOffset(b []byte) int {
	return (int(b[0]) + int(b[1]) + int(b[2]) + int(b[3])) % handshakeDigestOffsetRemainder
}

// 根据key生成buff的hs256哈希，offset是digest数据块的偏移（schema0/1）
func (h *Handshake) genDigest(buff []byte, hh hash.Hash, offset int) []byte {
	// 计算出数据块内的偏移+4+数据块整体的偏移 = digest在buff中的偏移
	offset = h.digestOffset(buff[offset:]) + 4 + offset
	hh.Reset()
	hh.Write(buff[handshakeData:offset])
	hh.Write(buff[offset+handshakeDigestLen:])
	digest := hh.Sum(nil)
	copy(buff[offset:], digest)
	return digest
}

// 检查数据，返回-1表示检查不通过，否则返回digest数据块在buff中的偏移
func (h *Handshake) checkDigest(buff []byte, hh hash.Hash) int {
	find := true
	var digest, check []byte
	// schema 0
	digestOffset := handshakeSchema0DigestBlock + h.digestOffset(buff[handshakeSchema0DigestBlock:])
	digestOffset2 := digestOffset + handshakeDigestLen
	hh.Reset()
	hh.Write(buff[handshakeData:digestOffset])
	hh.Write(buff[digestOffset2:])
	check = hh.Sum(nil)
	digest = buff[digestOffset:]
	for i := 0; i < len(check); i++ {
		if check[i] != digest[i] {
			find = false
			break
		}
	}
	if find {
		return handshakeSchema0DigestBlock
	}
	// schema 1
	digestOffset = handshakeSchema1DigestBlock + h.digestOffset(buff[handshakeSchema1DigestBlock:])
	digestOffset2 = digestOffset + handshakeDigestLen
	digest = buff[digestOffset:digestOffset2]
	hh.Reset()
	hh.Write(buff[handshakeData:digestOffset])
	hh.Write(buff[digestOffset2:])
	check = hh.Sum(nil)
	digest = buff[digestOffset:]
	for i := 0; i < len(check); i++ {
		if check[i] != digest[i] {
			find = false
			break
		}
	}
	if find {
		return handshakeSchema1DigestBlock
	}
	return -1
}
