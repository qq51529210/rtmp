package rtmp

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

type testConn struct {
	rc *sync.Cond
	wc *sync.Cond
	r  *bytes.Buffer
	w  *bytes.Buffer
}

func (c *testConn) Read(b []byte) (int, error) {
	c.rc.L.Lock()
	for c.r.Len() <= 0 {
		c.rc.Wait()
	}
	c.rc.L.Unlock()
	return c.r.Read(b)
}

func (c *testConn) Write(b []byte) (int, error) {
	n, err := c.w.Write(b)
	c.wc.Broadcast()
	return n, err
}

func TestHandshake(t *testing.T) {
	wait := new(sync.WaitGroup)
	wait.Add(2)
	network1 := sync.NewCond(new(sync.Mutex))
	network2 := sync.NewCond(new(sync.Mutex))
	data1 := bytes.NewBuffer(nil)
	data2 := bytes.NewBuffer(nil)
	client := &testConn{
		rc: network1,
		wc: network2,
		r:  data1,
		w:  data2,
	}
	server := &testConn{
		rc: network2,
		wc: network1,
		r:  data2,
		w:  data1,
	}
	go func() {
		defer wait.Done()
		_, err := HandshakeDial(client, 3)
		if err != nil {
			t.Log(err)
		}
	}()
	go func() {
		defer wait.Done()
		time.Sleep(time.Second)
		_, err := HandshakeAccept(server, 0)
		if err != nil {
			t.Log(err)
		}
	}()
	wait.Wait()
}
