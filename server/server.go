package main

import (
	"bufio"
	"net"
	"sync"

	"github.com/qq51529210/log"
)

type Server struct {
	Network           string
	Address           string
	listener          net.Listener
	running           bool
	WindowAckSize     uint32
	BandWidth         uint32
	BandWidthLimit    byte
	MaxChunkSize      int
	publishStreamLock sync.RWMutex
	publishStream     map[string]*Stream
}

func (s *Server) Listen() (err error) {
	s.listener, err = net.Listen(s.Network, s.Address)
	if err != nil {
		return
	}
	s.WindowAckSize = 1024 * 500
	s.BandWidth = 1024 * 500
	s.publishStream = make(map[string]*Stream)
	s.running = true
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Error(err)
			continue
		}
		go func(conn net.Conn) {
			c := new(Conn)
			c.server = s
			c.conn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
			err := c.MessageHandler.Read(c.conn, c.handleMessage)
			if err != nil {
				log.Error(err)
			}
			if c.publishStream != nil {
				s.DeleteStream(c.connectUrl.Path)
			}
		}(conn)
	}
	return
}

func (s *Server) GetPublishStream(name string) *Stream {
	s.publishStreamLock.RLock()
	stream := s.publishStream[name]
	s.publishStreamLock.RUnlock()
	return stream
}

func (s *Server) AddPublishStream(name string) (*Stream, bool) {
	s.publishStreamLock.Lock()
	stream, ok := s.publishStream[name]
	if !ok {
		stream = new(Stream)
		stream.Valid = true
	}
	s.publishStreamLock.Unlock()
	return stream, ok
}

func (s *Server) DeleteStream(name string) {
	s.publishStreamLock.Lock()
	stream, ok := s.publishStream[name]
	if ok {
		delete(s.publishStream, name)
		stream.Valid = false
	}
	s.publishStreamLock.Unlock()
}
