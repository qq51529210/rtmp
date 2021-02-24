package rtmp

import (
	"bytes"
	"testing"
)

func TestConn(t *testing.T) {
	var data bytes.Buffer
	c1 := NewConn(&data, GetMessage)
	c2 := NewConn(&data, GetMessage)
	msgTypeID := uint8(12)
	msgStreamID := uint32(34)
	msgTimestamp := uint32(MaxMessageTimestamp)
	msgExtTimestamp := uint32(MaxMessageTimestamp + 654321)
	csid := uint32(56)
	msgData := []byte("test")
	err := c1.WriteMessage(msgTypeID, msgStreamID, msgTimestamp, msgExtTimestamp, csid, msgData)
	if err != nil {
		t.Fatal(err)
	}
	var msg *Message
	msg, err = c2.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.TypeID != msgTypeID ||
		msg.StreamID != msgStreamID ||
		msg.Timestamp != msgExtTimestamp ||
		msg.Length != uint32(len(msgData)) ||
		bytes.Compare(msgData, msg.Data.Bytes()) != 0 {
		t.FailNow()
	}
}
