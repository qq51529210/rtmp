package rtmp

import (
	"encoding/binary"
	"io"
)

const (
	MaxMessageTimestamp = 0xffffff
	ChunkSize           = 128 // 默认的大小
)

type ChunkHeader struct {
	FMT               uint8
	CSID              uint32   // chunk stream id
	MessageTimestamp  uint32   // 3字节，在fmt1和fmt2中表示delta
	MessageLength     uint32   // 3字节
	MessageTypeID     uint8    // 1字节
	MessageStreamID   uint32   // 4字节
	ExtendedTimestamp uint32   // MessageTimestamp=0xffffff才会启用
	buff              [11]byte // basic3+message11+extended_timestamp4
}

// 从r中读取chunk header
func (c *ChunkHeader) Read(r io.Reader) (err error) {
	// basic header
	err = c.readBasicHeader(r)
	if err != nil {
		return
	}
	// message header
	err = c.readMessageHeader(r)
	if err != nil {
		return
	}
	// extended timestamp
	if c.MessageTimestamp == MaxMessageTimestamp {
		_, err = io.ReadFull(r, c.buff[:4])
		if err != nil {
			return
		}
		c.ExtendedTimestamp = binary.BigEndian.Uint32(c.buff[:])
	}
	return
}

// 解析chunk basic header
func (c *ChunkHeader) readBasicHeader(r io.Reader) (err error) {
	_, err = io.ReadFull(r, c.buff[:2])
	if err != nil {
		return
	}
	c.CSID = uint32(c.buff[0] & 0b00111111)
	switch c.CSID {
	case 0:
		// 2字节，[fmt+0][csid+64]
		c.CSID = uint32(c.buff[1]) + 64
	case 1:
		// 3字节，[fmt+1][csid+64][csid*256]
		_, err = io.ReadFull(r, c.buff[2:3])
		if err != nil {
			return
		}
		c.CSID = uint32(binary.LittleEndian.Uint16(c.buff[1:])) + 64
	default:
		// 1字节，[fmt+csid]
	}
	return
}

// 解析chunk message header
func (c *ChunkHeader) readMessageHeader(r io.Reader) (err error) {
	// message header
	switch c.buff[0] >> 6 {
	case 0:
		// 11字节
		_, err = io.ReadFull(r, c.buff[:])
		if err != nil {
			return
		}
		c.MessageTimestamp = bigEndianUint24(c.buff[0:])
		c.MessageLength = bigEndianUint24(c.buff[3:])
		c.MessageTypeID = c.buff[6]
		c.MessageStreamID = binary.LittleEndian.Uint32(c.buff[7:])
	case 1:
		// 7字节
		_, err = io.ReadFull(r, c.buff[:7])
		if err != nil {
			return
		}
		c.MessageTimestamp = bigEndianUint24(c.buff[0:])
		c.MessageLength = bigEndianUint24(c.buff[3:])
		c.MessageTypeID = c.buff[6]
	case 2:
		// 3字节
		_, err = io.ReadFull(r, c.buff[:3])
		if err != nil {
			return
		}
		c.MessageTimestamp = bigEndianUint24(c.buff[:3])
	default:
		// 没有message header
	}
	return
}

// 将chunk写到conn
func (c *ChunkHeader) Write(w io.Writer) (err error) {
	// basic header
	err = c.writeBasicHeader(w)
	if err != nil {
		return
	}
	// message header
	err = c.writeMessageHeader(w)
	if err != nil {
		return
	}
	// extended timestamp
	if c.MessageTimestamp == MaxMessageTimestamp {
		binary.BigEndian.PutUint32(c.buff[:], c.ExtendedTimestamp)
		_, err = w.Write(c.buff[:4])
	}
	return
}

// 编码chunk basic header
func (c *ChunkHeader) writeBasicHeader(w io.Writer) error {
	var n int
	// fmt
	c.buff[0] = c.FMT << 6
	// csid
	if c.CSID >= 64 {
		if c.CSID <= 319 {
			// 2字节，[fmt+0][csid+64]
			c.buff[1] = byte(c.CSID - 64)
			n = 2
		} else {
			// 3字节，[fmt+1][csid+64][csid*256]
			c.buff[0] |= 1
			binary.LittleEndian.PutUint16(c.buff[1:], uint16(c.CSID-64))
			n = 3
		}
	} else {
		// 1字节，[fmt+csid]
		c.buff[0] |= byte(c.CSID)
		n = 1
	}
	_, err := w.Write(c.buff[:n])
	return err
}

// 编码chunk message header
func (c *ChunkHeader) writeMessageHeader(w io.Writer) error {
	switch c.FMT {
	case 0:
		putBigEndianUint24(c.buff[0:], c.MessageTimestamp)
		putBigEndianUint24(c.buff[3:], c.MessageLength)
		c.buff[6] = c.MessageTypeID
		binary.LittleEndian.PutUint32(c.buff[7:], c.MessageStreamID)
		_, err := w.Write(c.buff[:11])
		return err
	case 1:
		putBigEndianUint24(c.buff[0:], c.MessageTimestamp)
		putBigEndianUint24(c.buff[3:], c.MessageLength)
		c.buff[6] = c.MessageTypeID
		_, err := w.Write(c.buff[:7])
		return err
	case 2:
		putBigEndianUint24(c.buff[0:], c.MessageTimestamp)
		_, err := w.Write(c.buff[:3])
		return err
	default:
		return nil
	}
}
