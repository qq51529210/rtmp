package main

import (
	"container/list"
	"sync"
)

var (
	StreamDataPool sync.Pool
)

func init() {
	StreamDataPool.New = func() interface{} {
		return new(StreamData)
	}
}

func PutStreamData(d *StreamData) {
	d.count--
	if d.count == 0 {
		d.Next = nil
		StreamDataPool.Put(d)
	}
}

// 表示一块音/视频数据
type StreamData struct {
	IsVideo   bool        // 音/视频
	Timestamp uint32      // 时间戳
	Data      []byte      // 数据
	Next      *StreamData // 下一块数据
	count     int32       // 智能指针
}

// 表示一块连续的音/视频数据块缓存
type Stream struct {
	lock      sync.RWMutex
	Valid     bool
	timestamp uint32
	head      *StreamData
	tail      *StreamData
	play      list.List
}

func (s *Stream) AddData(isVideo bool, timestamp uint32, data []byte) {
	b := StreamDataPool.Get().(*StreamData)
	b.count = 1
	b.IsVideo = isVideo
	b.Timestamp = timestamp
	b.Data = b.Data[:0]
	b.Data = append(b.Data, data...)
	s.lock.Lock()
	if s.head == nil {
		s.head = b
		s.tail = b
	} else {
		if s.tail != nil {
			s.tail.Next = b
		}
		s.tail = b
	}
	if b.Timestamp-s.head.Timestamp >= s.timestamp {
		h := s.head
		s.head = s.head.Next
		for ele := s.play.Front(); ele != nil; ele = ele.Next() {
			conn := ele.Value.(*Conn)
			select {
			case conn.playChan <- h:
			default:
			}
		}
	}
	s.tail.Next = b
	s.lock.Unlock()
}

func (s *Stream) AddPlayConn(c *Conn) {
	s.lock.Lock()
	for ele := s.play.Front(); ele != nil; ele = ele.Next() {
		conn := ele.Value.(*Conn)
		if conn == c {
			s.lock.Unlock()
			return
		}
	}
	s.play.PushBack(c)
	s.lock.Unlock()
}

func (s *Stream) RemovePlayConn(c *Conn) {
	s.lock.Lock()
	for ele := s.play.Front(); ele != nil; ele = ele.Next() {
		conn := ele.Value.(*Conn)
		if conn == c {
			s.play.Remove(ele)
			close(conn.playChan)
			s.lock.Unlock()
			return
		}
	}
	s.lock.Unlock()
}
