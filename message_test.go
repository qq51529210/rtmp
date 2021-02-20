package rtmp

import (
	"bytes"
	"testing"
)

func TestMessage(t *testing.T) {
	var b bytes.Buffer
	var m1 Message
	m1.CSID = 123
	m1.Timestamp = 123456678
	m1.TypeID = 12
	m1.StreamID = 1234
	m1.Data.WriteString("test")
	m1.Write(&b, 8)

	var r MessageReader
	r.Init()
	m2, err := r.Read(&b)
	if err != nil {
		t.Fatal(err)
	}
	if m2.CSID != m1.CSID ||
		m2.Timestamp != m1.Timestamp ||
		m2.StreamID != m1.StreamID ||
		string(m2.Data.Bytes()) != "test" {
		t.FailNow()
	}
}
