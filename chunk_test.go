package rtmp

import (
	"bytes"
	"testing"
)

func TestChunkHeader(t *testing.T) {
	var b bytes.Buffer
	var c1, c2 ChunkHeader
	c1.FMT = 0
	c1.CSID = 11232
	c1.MessageTimestamp = 123451231
	c1.MessageLength = 1234
	c1.MessageTypeID = 32
	c1.MessageStreamID = 123
	err := c1.Write(&b)
	if err != nil {
		t.Fatal(err)
	}
	err = c2.Read(&b)
	if err != nil {
		t.Fatal(err)
	}
	if c2.FMT != c1.FMT ||
		c2.CSID != c1.CSID ||
		c2.MessageTimestamp != c1.MessageTimestamp ||
		c2.MessageLength != c1.MessageLength ||
		c2.MessageTypeID != c1.MessageTypeID ||
		c2.MessageStreamID != c1.MessageStreamID {
		t.FailNow()
	}
}
