package main

import (
	"bufio"
	"net"
	"sync"

	"github.com/qq51529210/log"
)

type Server struct {
	Network        string
	Address        string
	listener       net.Listener
	running        bool
	streamMutex    sync.RWMutex
	stream         map[string]*AVStream
	WindowAckSize  uint32
	BandWidth      uint32
	BandWidthLimit byte
	MaxChunkSize   int
}

func (s *Server) Listen() (err error) {
	s.listener, err = net.Listen(s.Network, s.Address)
	if err != nil {
		return
	}
	s.WindowAckSize = 1024 * 500
	s.BandWidth = 1024 * 500
	s.stream = make(map[string]*AVStream)
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
			err := c.MessageReader.Read(c.conn, c.handleMessage)
			if err != nil {
				log.Error(err)
			}
		}(conn)
	}
	return
}

func (s *Server) GetStream(name string) *AVStream {
	s.streamMutex.RLock()
	stream := s.stream[name]
	s.streamMutex.RUnlock()
	return stream
}

func (s *Server) AddStream(name string) (*AVStream, bool) {
	s.streamMutex.Lock()
	stream, ok := s.stream[name]
	if !ok {
		stream = new(AVStream)
		stream.Valid = true
	}
	s.streamMutex.Unlock()
	return stream, ok
}
