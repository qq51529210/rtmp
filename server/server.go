package main

import (
	"bufio"
	"net"
	"sync"

	"github.com/qq51529210/log"
	"github.com/qq51529210/rtmp"
)

type Server struct {
	Address           string
	listener          net.Listener
	running           bool
	WindowAckSize     uint32
	BandWidth         uint32
	BandWidthLimit    byte
	MaxChunkSize      int
	Version           uint32
	publishStreamLock sync.RWMutex
	publishStream     map[string]*Stream
}

func (s *Server) Listen() (err error) {
	s.listener, err = net.Listen("tcp", s.Address)
	if err != nil {
		return
	}
	s.WindowAckSize = 1024 * 500
	s.BandWidth = 1024 * 500
	s.BandWidthLimit = 2
	s.publishStream = make(map[string]*Stream)
	s.running = true
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Error(err)
			continue
		}
		go s.ServeConn(conn)
	}
	return
}

func (s *Server) ServeConn(conn net.Conn) {
	c := new(Conn)
	c.server = s
	c.conn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	defer func() {
		if c.publishStream != nil {
			s.DeleteStream(c.connectUrl.Path)
		}
		conn.Close()
	}()
	_, err := rtmp.HandshakeAccept(conn, s.Version)
	if err != nil {
		log.Error(err)
		return
	}
	var msg *rtmp.Message
	for {
		msg, err = c.MessageReader.Read(c.conn)
		if err != nil {
			log.Error(err)
			return
		}
		err = c.handleMessage(msg)
		if err != nil {
			log.Error(err)
			return
		}
	}
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
