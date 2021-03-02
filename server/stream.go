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

func PutStreamData(data *StreamData) {
	data.referrence--
	if data.referrence <= 0 {
		StreamDataPool.Put(data)
	}
}

func GetStreamData(msg *rtmp.Message) *StreamData {
	data := StreamDataPool.Get().(*StreamData)
	data.typeID = msg.TypeID
	data.timestamp = msg.Timestamp
	data.data.Reset()
	data.data.Write(msg.Data.Bytes())
	return data
}

// 表示一块音/视频数据
type StreamData struct {
	typeID     byte         // 音/视频
	timestamp  uint32       // 时间戳
	data       bytes.Buffer // 数据
	referrence int          // 引用的次数
}

// 表示一块连续的音/视频数据块缓存
type Stream struct {
	lock     sync.Mutex
	valid    bool
	dataConn chan *StreamData
	playConn list.List
	metaData bytes.Buffer
	avc      *StreamData // 第一个avc，包含sps pps
	acc      *StreamData
}

func newStream() *Stream {
	stream := new(Stream)
	stream.valid = true
	stream.dataConn = make(chan *StreamData, 1)
	go stream.Broadcast()
	return stream
}

func (s *Stream) AddVideo(msg *rtmp.Message) {
	data := GetStreamData(msg)
	if s.avc == nil {
		s.avc = data
	} else {
		select {
		case s.dataConn <- data:
		default:
			PutStreamData(data)
		}
	}
}

func (s *Stream) AddAudio(msg *rtmp.Message) {
	data := GetStreamData(msg)
	if s.acc == nil {
		s.acc = data
	} else {
		select {
		case s.dataConn <- data:
		default:
			PutStreamData(data)
		}
	}
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

func (s *Stream) Broadcast() {
	for s.valid {
		data, ok := <-s.dataConn
		if !ok {
			return
		}
		s.lock.Lock()
		data.referrence += s.playConn.Len()
		for ele := s.playConn.Front(); ele != nil; ele = ele.Next() {
			conn := ele.Value.(*Conn)
			select {
			case conn.playChan <- data:
			default:
				data.referrence--
			}
		}
		s.lock.Unlock()
	}
}
