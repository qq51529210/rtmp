package rtmp

import (
	"encoding/binary"
	"io"
)

// [..min......max....]
type buffer struct {
	buf []byte // 数据
	min int    // 未读的数据的索引
	max int    // 有效数据的最大索引
}

// 返回缓存中字节总数
func (b *buffer) Len() int {
	return b.max - b.min
}

// 重置
func (b *buffer) Reset() {
	b.min = 0
	b.max = 0
}

// 确保缓存可以添加n个字节
func (b *buffer) GrowFree(n int) {
	n1 := len(b.buf) - b.max
	if n1 < n {
		n2 := cap(b.buf) - b.max
		if n2 < n {
			nb := make([]byte, b.max+n)
			copy(nb, b.buf)
			b.buf = nb
		} else {
			b.buf = b.buf[:b.max+n]
		}
	}
}

// io.Reader
func (b *buffer) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, io.ErrShortBuffer
	}
	if b.max == b.min {
		return 0, io.EOF
	}
	n := copy(buf, b.buf[b.min:b.max])
	b.min += n
	return n, nil
}

// io.Writer
func (b *buffer) Write(buf []byte) (int, error) {
	m := b.max
	b.GrowFree(len(buf))
	var n int
	for len(buf) > 0 {
		n = copy(b.buf[b.max:], buf)
		b.max += n
		buf = buf[n:]
	}
	return b.max - m, nil
}

// 确保缓存中至少有n个字节
func (b *buffer) ReadAtLeast(conn io.Reader, n int) error {
	m := n - b.Len()
	if m > 0 {
		b.GrowFree(m)
		var err error
		m, err = io.ReadAtLeast(conn, b.buf[b.max:], m)
		if err != nil {
			return err
		}
		b.max += m
	}
	return nil
}

func (b *buffer) Data() []byte {
	return b.buf[b.min:b.max]
}

func (b *buffer) WriteBytes(data []byte) {
	n := copy(b.buf[b.max:], data)
	b.max += n
	if n < len(data) {
		b.buf = append(b.buf, data[n:]...)
	}
}

func (b *buffer) ReadByte() byte {
	n := b.buf[b.min]
	b.min++
	return n
}

func (b *buffer) WriteByte(n byte) {
	b.buf[b.max] = n
	b.max++
}

func (b *buffer) WriteBigEndianUint32(n uint32) {
	b.GrowFree(4)
	binary.BigEndian.PutUint32(b.buf[b.max:], n)
	b.max += 4
}

func (b *buffer) WriteLittleEndianUint32(n uint32) {
	b.GrowFree(4)
	binary.LittleEndian.PutUint32(b.buf[b.max:], n)
	b.max += 4
}

func (b *buffer) WriteBigEndianUint24(n uint32) {
	b.GrowFree(3)
	putBigEndianUint24(b.buf[b.max:], n)
	b.max += 3
}

func (b *buffer) WriteLittleEndianUint24(n uint32) {
	b.GrowFree(3)
	putLittleEndianUint24(b.buf[b.max:], n)
	b.max += 3
}

func (b *buffer) ReadBigEndianUint32() uint32 {
	n := binary.BigEndian.Uint32(b.buf[b.min:])
	b.min += 4
	return n
}

func (b *buffer) ReadLittleEndianUint32() uint32 {
	n := binary.LittleEndian.Uint32(b.buf[b.min:])
	b.min += 4
	return n
}

func (b *buffer) ReadBigEndianUint24() uint32 {
	n := bigEndianUint24(b.buf[b.min:])
	b.min += 3
	return n
}

func (b *buffer) ReadLittleEndianUint24() uint32 {
	n := littleEndianUint24(b.buf[b.min:])
	b.min += 3
	return n
}
