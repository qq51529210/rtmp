package main

import (
	"bufio"
	"net"
	"sync"

	"github.com/qq51529210/log"
	"github.com/qq51529210/rtmp"
)

type Server struct {
	Address               string
	listener              net.Listener
	running               bool
	Timestamp             uint32
	WindowAcknowledgeSize uint32
	BandWidth             uint32
	BandWidthLimit        byte
	ChunkSize             uint32
	Version               uint32
	publishStreamLock     sync.RWMutex
	publishStream         map[string]*Stream
}

func (s *Server) Listen() (err error) {
	s.listener, err = net.Listen("tcp", s.Address)
	if err != nil {
		return
	}
	s.Timestamp = 1000 * 2
	s.WindowAcknowledgeSize = 1024 * 500
	s.BandWidth = 1024 * 500
	s.BandWidthLimit = 2
	s.ChunkSize = 4 * 1024
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
	c.receiveAudio = true
	c.receiveVideo = true
	c.reader = bufio.NewReader(conn)
	c.writer = conn
	defer func() {
		if c.publishStream != nil {
			s.DeleteStream(c.connectUrl.Path)
		}
		conn.Close()
	}()
	_, err := rtmp.HandshakeAccept(conn, s.Version)
	if err != nil {
		log.Error(err)
	}
	err = c.readLoop()
	if err != nil {
		log.Error(err)
		return
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
		s.publishStream[name] = stream
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
