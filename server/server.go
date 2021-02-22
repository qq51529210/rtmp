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
	Timestamp         uint32
	AckSize           uint32
	BandWidth         uint32
	BandWidthLimit    byte
	MaxChunkSize      uint32
	Version           uint32
	publishStreamLock sync.RWMutex
	publishStream     map[string]*Stream
}

func (s *Server) Listen() (err error) {
	s.listener, err = net.Listen("tcp", s.Address)
	if err != nil {
		return
	}
	s.Timestamp = 1000 * 2
	s.AckSize = 1024 * 500
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
	log.Debug(conn.RemoteAddr().String())
	c := new(Conn)
	c.server = s
	c.conn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	c.MessageReader.Init(s.AckSize, s.BandWidth, s.BandWidthLimit)
	defer func() {
		if c.publishStream != nil {
			s.DeleteStream(c.connectUrl.Path)
		}
		conn.Close()
	}()
	// 验证是不通过，但是不影响
	rtmp.HandshakeAccept(conn, s.Version)
	err := c.MessageReader.ReadLoop(c.conn, c.handleMessage)
	if err != nil {
		log.Error(err)
	}
}

func (s *Server) GetPublishStream(name string) *Stream {
	s.publishStreamLock.RLock()
	stream := s.publishStream[name]
	s.publishStreamLock.RUnlock()
	return stream
}

func (s *Server) AddPublishStream(name string, timestamp uint32) (*Stream, bool) {
	s.publishStreamLock.Lock()
	stream, ok := s.publishStream[name]
	if !ok {
		stream = new(Stream)
		stream.timestamp = timestamp
		stream.Valid = true
	}
	s.publishStreamLock.Unlock()
	return stream, !ok
}

func (s *Server) DeleteStream(name string) {
	s.publishStreamLock.Lock()
	stream, ok := s.publishStream[name]
	if ok {
		delete(s.publishStream, name)
		stream.Valid = false
	}
	s.publishStreamLock.Unlock()
	go func(stream *Stream) {
		if stream == nil {
			return
		}
		b := stream.head
		for b != nil {
			PutStreamData(b)
			b = b.Next
		}
	}(stream)
}
