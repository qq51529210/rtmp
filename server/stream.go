package main

import (
	"sync"
	"sync/atomic"
)

var (
	avBlockPool sync.Pool
)

func init() {
	avBlockPool.New = func() interface{} {
		return new(AVBlock)
	}
}

// 表示一块音/视频数据
type AVBlock struct {
	IsVideo   bool     // 音/视频
	Timestamp uint32   // 时间戳
	Data      []byte   // 数据
	Next      *AVBlock // 下一块数据
	count     int32    // 智能指针
}

func (b *AVBlock) Done() {
	if atomic.AddInt32(&b.count, -1) == 0 {
		avBlockPool.Put(b)
	}
}

// 表示一块连续的音/视频数据块缓存
type AVStream struct {
	lock      *sync.Cond
	Valid     bool
	timestamp uint32
	head      *AVBlock
	tail      *AVBlock
}

func (s *AVStream) GetData(prev *AVBlock) *AVBlock {
	if prev != nil {
		prev.Done()
	}
	s.lock.L.Lock()
	for prev == s.tail {
		s.lock.Wait()
	}
	s.lock.L.Unlock()
	atomic.AddInt32(&s.tail.count, 1)
	return s.tail
}

func (s *AVStream) AddData(isVideo bool, timestamp uint32, data []byte) {
	b := avBlockPool.Get().(*AVBlock)
	b.count = 1
	b.IsVideo = isVideo
	b.Timestamp = timestamp
	b.Data = b.Data[:0]
	b.Data = append(b.Data, data...)
	s.lock.L.Lock()
	if b.Timestamp-s.head.Timestamp >= s.timestamp {
		h := s.head
		s.head = s.head.Next
		h.Done()
		s.lock.Broadcast()
	}
	s.tail.Next = b
	s.lock.L.Unlock()
}
