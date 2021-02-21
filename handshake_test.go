package rtmp

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

type testConn struct {
	q chan struct{}
	w chan []byte
	r chan []byte
	b bytes.Buffer
}

func (c *testConn) Read(b []byte) (int, error) {
	if c.b.Len() > 0 {
		return c.b.Read(b)
	}
	select {
	case d := <-c.r:
		c.b.Write(d)
	case <-c.q:
		return 0, io.EOF
	}
	return c.b.Read(b)
}

func (c *testConn) Write(b []byte) (int, error) {
	d := make([]byte, len(b))
	copy(d, b)
	select {
	case c.w <- d:
		return len(d), nil
	case <-c.q:
		return 0, io.ErrShortWrite
	}
}

func TestHandshake(t *testing.T) {
	wait := new(sync.WaitGroup)
	wait.Add(2)
	quit := make(chan struct{})
	once := new(sync.Once)
	c1 := make(chan []byte, 1)
	c2 := make(chan []byte, 1)
	client := new(testConn)
	client.q = quit
	client.w = c1
	client.r = c2
	server := new(testConn)
	server.q = quit
	server.w = client.r
	server.r = client.w
	var err1, err2 error
	go func() {
		defer wait.Done()
		_, err1 = HandshakeDial(client, 3)
		if err1 != nil {
			once.Do(func() { close(quit) })
		}
	}()
	go func() {
		defer wait.Done()
		time.Sleep(time.Millisecond * 10)
		_, err2 = HandshakeAccept(server, 0)
		if err2 != nil {
			once.Do(func() { close(quit) })
		}
	}()
	wait.Wait()
	once.Do(func() { close(quit) })
	if err1 != nil {
		t.Fatal(err1)
	}
	if err2 != nil {
		t.Fatal(err2)
	}
}
