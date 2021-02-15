package rtmp

const (
	MinStreamId         = 3
	MaxStreamId         = 65599
	MaxMessageTimestamp = 0xffffff
	ChunkSize           = 128 // 默认的大小
)

func putUint24(b []byte, n uint32) {
	b[0] = byte(n >> 16)
	b[1] = byte(n >> 8)
	b[2] = byte(n)
}

func uint24(b []byte) uint32 {
	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
}

type Stream struct {
	Chunk map[uint32]*Chunk
}

// type buffer struct {
// 	buff   []byte
// 	i1, i2 int
// }

// func (b *buffer) ReadAtLeast(c io.Reader, n int) (err error) {
// 	m := b.i2 - b.i1
// 	if m < n {
// 		m, err = io.ReadAtLeast(c, b.buff[b.i2:], n-m)
// 		if err != nil {
// 			return
// 		}
// 		b.i2 += m
// 	}
// 	return
// }
