package rtmp

import (
	"encoding/binary"
	"io"
)

type Chunk struct {
	FMT               uint8
	CSID              uint32 // chunk stream id
	MessageTimestamp  uint32 // 3字节，在fmt1和fmt2中表示delta
	MessageLength     uint32 // 3字节
	MessageTypeID     uint8  // 1字节
	MessageStreamID   uint32 // 4字节
	ExtendedTimestamp uint32 // MessageTimestamp=0xffffff才会启用
	Data              []byte
	buff              [16]byte
	idx1              int
	idx2              int
}

// 从conn中读取一个完整的chunk
func (c *Chunk) Read(conn io.Reader) (err error) {
	c.idx1 = 0
	c.idx2 = 0
	// basic header
	err = c.readBasicHeader(conn)
	if err != nil {
		return
	}
	// message header
	err = c.readMessageHeader(conn)
	if err != nil {
		return
	}
	// extended timestamp
	if c.MessageTimestamp == MaxMessageTimestamp {
		err = c.readAtLeast(conn, 4)
		if err != nil {
			return
		}
		c.ExtendedTimestamp = binary.BigEndian.Uint32(c.buff[c.idx1:])
		c.idx1 += 4
	}
	// chunk data
	if c.idx2 > c.idx1 {
		_, err = io.ReadFull(conn, c.Data[copy(c.Data, c.buff[c.idx1:c.idx2]):])
	} else {
		_, err = io.ReadFull(conn, c.Data)
	}
	return
}

// 确保缓存中至少有n个字节
func (c *Chunk) readAtLeast(conn io.Reader, n int) error {
	m := c.idx2 - c.idx1
	if m < n {
		m, err := io.ReadAtLeast(conn, c.buff[c.idx2:], n-m)
		if err != nil {
			return err
		}
		c.idx2 += m
	}
	return nil
}

// 解析chunk basic header
func (c *Chunk) readBasicHeader(conn io.Reader) (err error) {
	c.idx2, err = conn.Read(c.buff[:])
	if err != nil {
		return
	}
	c.CSID = uint32(c.buff[0] & 0b00111111)
	switch c.CSID {
	case 0:
		// 2字节，[fmt+0][csid+64]
		err = c.readAtLeast(conn, 2)
		if err != nil {
			return
		}
		c.CSID = uint32(c.buff[1]) + 64
		c.idx1 = 2
	case 1:
		// 3字节，[fmt+1][csid+64][csid*256]
		err = c.readAtLeast(conn, 3)
		if err != nil {
			return
		}
		c.CSID = uint32(c.buff[1]) + 64 + uint32(c.buff[2])*256
		c.idx1 = 3
	default:
		// 1字节，[fmt+csid]
		c.idx1 = 1
	}
	return
}

// 解析chunk message header
func (c *Chunk) readMessageHeader(conn io.Reader) (err error) {
	// message header
	switch c.buff[0] >> 6 {
	case 0:
		// 11字节
		err = c.readAtLeast(conn, 11)
		if err != nil {
			return
		}
		c.MessageTimestamp = uint24(c.buff[c.idx1:])
		c.idx1 += 3
		c.MessageLength = uint24(c.buff[c.idx1:])
		c.idx1 += 3
		c.MessageTypeID = c.buff[c.idx1]
		c.idx1++
		c.MessageStreamID = binary.LittleEndian.Uint32(c.buff[c.idx1:])
		c.idx1 += 4
	case 1:
		// 7字节
		err = c.readAtLeast(conn, 7)
		if err != nil {
			return
		}
		c.MessageTimestamp = uint24(c.buff[c.idx1:])
		c.idx1 += 3
		c.MessageLength = uint24(c.buff[c.idx1:])
		c.idx1 += 3
		c.MessageTypeID = c.buff[c.idx1]
		c.idx1++
	case 2:
		// 3字节
		err = c.readAtLeast(conn, 3)
		if err != nil {
			return
		}
		c.MessageTimestamp = uint24(c.buff[c.idx1:])
		c.idx1 += 3
	default:
		// 没有message header
		c.MessageTimestamp = 0
	}
	return
}

// 将chunk写到conn
func (c *Chunk) Write(conn io.Writer) (err error) {
	c.idx2 = 0
	// basic header
	c.writeBasicHeader()
	// message header
	c.writeMessageHeader()
	// extended timestamp
	if c.MessageTimestamp == MaxMessageTimestamp {
		binary.BigEndian.PutUint32(c.buff[c.idx2:], c.ExtendedTimestamp)
		c.idx2 += 4
	}
	// chun header
	_, err = conn.Write(c.buff[:c.idx2])
	if err != nil {
		return
	}
	// chunk data
	_, err = conn.Write(c.Data)
	return
}

// 编码chunk basic header
func (c *Chunk) writeBasicHeader() {
	// fmt
	c.buff[0] = c.FMT << 6
	// csid
	if c.CSID >= 64 {
		if c.CSID <= 319 {
			// 2字节，[fmt+0][csid+64]
			c.buff[1] = byte(c.CSID - 64)
			c.idx2 = 2
		} else {
			// 3字节，[fmt+1][csid+64][csid*256]
			c.buff[0] |= 1
			c.buff[2] = byte(c.CSID / 256)
			c.buff[1] = byte(c.CSID - uint32(c.buff[2])*256 - 64)
			c.idx2 = 3
		}
	} else {
		// 1字节，[fmt+csid]
		c.buff[0] |= byte(c.CSID)
		c.idx2 = 1
	}
	return
}

// 编码chunk message header
func (c *Chunk) writeMessageHeader() {
	switch c.FMT {
	case 0:
		putUint24(c.buff[c.idx2:], c.MessageTimestamp)
		c.idx2 += 3
		putUint24(c.buff[c.idx2:], c.MessageLength)
		c.idx2 += 3
		c.buff[c.idx2] = c.MessageTypeID
		c.idx2++
		binary.LittleEndian.PutUint32(c.buff[c.idx2:], c.MessageStreamID)
		c.idx2 += 4
	case 1:
		putUint24(c.buff[c.idx2:], c.MessageTimestamp)
		c.idx2 += 3
		putUint24(c.buff[c.idx2:], c.MessageLength)
		c.idx2 += 3
		c.buff[c.idx2] = c.MessageTypeID
		c.idx2++
	case 2:
		putUint24(c.buff[c.idx2:], c.MessageTimestamp)
		c.idx2 += 3
	default:
	}
}
