package main

import (
	"bytes"
	"container/list"
	"sync"

	"github.com/qq51529210/rtmp"
)

var (
	StreamDataPool sync.Pool
)

func init() {
	StreamDataPool.New = func() interface{} {
		return new(StreamData)
	}
}

// 表示一块音/视频数据
type StreamData struct {
	typeID    byte         // 音/视频
	timestamp uint32       // 时间戳
	data      bytes.Buffer // 数据
	next      *StreamData  // 下一块数据
}

type StreamGOP struct {
	data  list.List // 数据
	count int32     // 智能指针
}

func (s *StreamGOP) Release() {
	s.count--
	if s.count > 0 {
		return
	}
	s.recovery()
}

func (s *StreamGOP) recovery() {
	for ele := s.data.Front(); ele != nil; ele = ele.Next() {
		StreamDataPool.Put(ele.Value)
	}
}

func newStreamGOP(data interface{}) {
	p := new(StreamGOP)
	p.count = 1
	p.data.PushBack(data)
}

// 表示一块连续的音/视频数据块缓存
type Stream struct {
	lock     sync.Mutex
	valid    bool
	dataConn chan *StreamGOP
	playConn list.List
	metaData bytes.Buffer
	gop      *StreamGOP
	avc      *StreamData
	acc      *StreamData
}

func (s *Stream) AddData(msg *rtmp.Message) {
	data := StreamDataPool.Get().(*StreamData)
	data.next = nil
	data.typeID = msg.TypeID
	data.timestamp = msg.Timestamp
	data.data.Reset()
	data.data.Write(msg.Data.Bytes())

	if s.gop == nil {
		s.gop = new(StreamGOP)
	}
	p := msg.Data.Bytes()
	if p[0] == 0x17 {
		// 关键帧
		gop := s.gop
		select {
		case s.dataConn <- gop:
		default:
			gop.recovery()
		}
		s.gop = new(StreamGOP)
	}
	s.gop.data.PushBack(data)
}

func (s *Stream) AddPlayConn(c *Conn) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for ele := s.playConn.Front(); ele != nil; ele = ele.Next() {
		conn := ele.Value.(*Conn)
		if conn == c {
			return
		}
	}
	s.playConn.PushBack(c)
}

func (s *Stream) RemovePlayConn(c *Conn) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for ele := s.playConn.Front(); ele != nil; ele = ele.Next() {
		conn := ele.Value.(*Conn)
		if conn == c {
			s.playConn.Remove(ele)
			close(conn.playChan)
			return
		}
	}
}

func (s *Stream) Play() {
	for s.valid {
		data, ok := <-s.dataConn
		if !ok {
			return
		}
		s.lock.Lock()
		for ele := s.playConn.Front(); ele != nil; ele = ele.Next() {
			conn := ele.Value.(*Conn)
			select {
			case conn.playChan <- data:
				data.count++
			default:
			}
		}
		s.lock.Unlock()
	}
}
