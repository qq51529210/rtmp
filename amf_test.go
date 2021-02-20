package rtmp

import (
	"bytes"
	"testing"
)

func TestAMF(t *testing.T) {
	var b bytes.Buffer
	WriteAMF(&b, "connect")
	WriteAMF(&b, 1)
	WriteAMF(&b, nil)
	WriteAMF(&b, true)
	WriteAMF(&b, map[string]interface{}{
		"a": "A",
		"b": true,
		"c": 2,
		"d": nil,
		"e": map[string]interface{}{
			"a": 1,
		},
	})
	amf, err := ReadAMF(&b)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := amf.(string)
	if !ok || s != "connect" {
		t.FailNow()
	}
	amf, err = ReadAMF(&b)
	if err != nil {
		t.Fatal(err)
	}
	n, ok := amf.(float64)
	if !ok || n != 1 {
		t.FailNow()
	}
	amf, err = ReadAMF(&b)
	if err != nil {
		t.Fatal(err)
	}
	if amf != nil {
		t.FailNow()
	}
	amf, err = ReadAMF(&b)
	if err != nil {
		t.Fatal(err)
	}
	o, ok := amf.(bool)
	if !ok || !o {
		t.FailNow()
	}
	amf, err = ReadAMF(&b)
	if err != nil {
		t.Fatal(err)
	}
	ob, ok := amf.(map[string]interface{})
	if !ok {
		t.FailNow()
	}
	s, ok = ob["a"].(string)
	if !ok || s != "A" {
		t.FailNow()
	}
	o, ok = ob["b"].(bool)
	if !ok || !o {
		t.FailNow()
	}
	n, ok = ob["c"].(float64)
	if !ok || n != 2 {
		t.FailNow()
	}
	if ob["d"] != nil {
		t.FailNow()
	}
	ob, ok = ob["e"].(map[string]interface{})
	if !ok {
		t.FailNow()
	}
	n, ok = ob["a"].(float64)
	if !ok || n != 1 {
		t.FailNow()
	}
}
