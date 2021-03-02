package rtmp

import (
	"encoding/binary"
	"io"
)

const (
	MaxMessageTimestamp = 0xffffff
	MaxChunkSize        = 0xffffff
	ChunkSize           = 128 // 默认的大小
)

const (
	ChunkFmt0 = iota
	ChunkFmt1
	ChunkFmt2
	ChunkFmt3
)

type ChunkHeader struct {
	FMT               uint8    // 0:11，1:7，2:3，3:0
	CSID              uint32   // chunk stream id
	MessageTimestamp  uint32   // 3字节，在fmt1和fmt2中表示delta
	MessageLength     uint32   // 3字节
	MessageTypeID     uint8    // 1字节
	MessageStreamID   uint32   // 4字节
	ExtendedTimestamp uint32   // 4字节
	buff              [11]byte // basic:3+message:11+extended_timestamp:4
}

func (c *ChunkHeader) Read(r io.Reader) (err error) {
	// basic
	err = c.ReadBaisc(r)
	if err != nil {
		return
	}
	// message
	err = c.ReadMessage(r)
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

func (c *ChunkHeader) ReadBaisc(r io.Reader) (err error) {
	// fmt
	_, err = io.ReadFull(r, c.buff[:1])
	if err != nil {
		return
	}
	c.FMT = c.buff[0] >> 6
	// csid
	c.CSID = uint32(c.buff[0] & 0b00111111)
	switch c.CSID {
	case 0:
		// 2字节，[fmt+0][csid+64]
		_, err = io.ReadFull(r, c.buff[1:2])
		if err != nil {
			return
		}
		c.CSID = uint32(c.buff[1]) + 64
	case 1:
		// 3字节，[fmt+1][csid+64][csid*256]
		_, err = io.ReadFull(r, c.buff[1:3])
		if err != nil {
			return
		}
		c.CSID = uint32(binary.LittleEndian.Uint16(c.buff[1:])) + 64
	default:
		// 1字节，[fmt+csid]
	}
	return
}

func (c *ChunkHeader) ReadMessage(r io.Reader) (err error) {
	switch c.FMT {
	case ChunkFmt0:
		// 11字节
		_, err = io.ReadFull(r, c.buff[:])
		if err != nil {
			return
		}
		c.MessageTimestamp = bUint24(c.buff[0:])
		c.MessageLength = bUint24(c.buff[3:])
		c.MessageTypeID = c.buff[6]
		c.MessageStreamID = binary.LittleEndian.Uint32(c.buff[7:])
	case ChunkFmt1:
		// 7字节
		_, err = io.ReadFull(r, c.buff[:7])
		if err != nil {
			return
		}
		c.MessageTimestamp = bUint24(c.buff[0:])
		c.MessageLength = bUint24(c.buff[3:])
		c.MessageTypeID = c.buff[6]
	case ChunkFmt2:
		// 3字节
		_, err = io.ReadFull(r, c.buff[:3])
		if err != nil {
			return
		}
		c.MessageTimestamp = bUint24(c.buff[0:])
	}
	return
}

func (c *ChunkHeader) ReadExtendedTimestamp(r io.Reader) (err error) {
	_, err = io.ReadFull(r, c.buff[:4])
	if err != nil {
		return
	}
	c.ExtendedTimestamp = binary.BigEndian.Uint32(c.buff[:4])
	return
}

func (c *ChunkHeader) ReadFmt0Message(r io.Reader) (err error) {
	// 11字节
	_, err = io.ReadFull(r, c.buff[:11])
	if err != nil {
		return
	}
	c.MessageTimestamp = bUint24(c.buff[0:])
	c.MessageLength = bUint24(c.buff[3:])
	c.MessageTypeID = c.buff[6]
	c.MessageStreamID = binary.LittleEndian.Uint32(c.buff[7:])
	return
}

func (c *ChunkHeader) ReadFmt1Message(r io.Reader) (err error) {
	// 7字节
	_, err = io.ReadFull(r, c.buff[:7])
	if err != nil {
		return
	}
	c.MessageTimestamp = bUint24(c.buff[0:])
	c.MessageLength = bUint24(c.buff[3:])
	c.MessageTypeID = c.buff[6]
	return
}

func (c *ChunkHeader) ReadFmt2Message(r io.Reader) (err error) {
	// 3字节
	_, err = io.ReadFull(r, c.buff[:3])
	if err != nil {
		return
	}
	c.MessageTimestamp = bUint24(c.buff[0:])
	return
}

func (c *ChunkHeader) Write(w io.Writer) (err error) {
	// fmt
	c.buff[0] = c.FMT << 6
	// csid
	if c.CSID >= 64 {
		if c.CSID <= 319 {
			// 2字节，[fmt+0][csid+64]
			c.buff[1] = byte(c.CSID - 64)
			_, err = w.Write(c.buff[:2])
		} else {
			// 3字节，[fmt+1][csid+64][csid*256]
			c.buff[0] |= 1
			binary.LittleEndian.PutUint16(c.buff[1:], uint16(c.CSID-64))
			_, err = w.Write(c.buff[:3])
		}
	} else {
		// 1字节，[fmt+csid]
		c.buff[0] |= byte(c.CSID)
		_, err = w.Write(c.buff[:1])
	}
	if err != nil {
		return
	}
	// message
	switch c.FMT {
	case ChunkFmt0:
		putBUint24(c.buff[0:], c.MessageTimestamp)
		putBUint24(c.buff[3:], c.MessageLength)
		c.buff[6] = c.MessageTypeID
		binary.LittleEndian.PutUint32(c.buff[7:], c.MessageStreamID)
		_, err = w.Write(c.buff[:11])
		if err != nil {
			return
		}
	case ChunkFmt1:
		putBUint24(c.buff[0:], c.MessageTimestamp)
		putBUint24(c.buff[3:], c.MessageLength)
		c.buff[6] = c.MessageTypeID
		_, err = w.Write(c.buff[:7])
		if err != nil {
			return
		}
	case ChunkFmt2:
		putBUint24(c.buff[0:], c.MessageTimestamp)
		_, err = w.Write(c.buff[:3])
		if err != nil {
			return
		}
	}
	// extended timestamp
	if c.ExtendedTimestamp > 0 {
		binary.BigEndian.PutUint32(c.buff[:], c.ExtendedTimestamp)
		_, err = w.Write(c.buff[:4])
	}
	return
}
