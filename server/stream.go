package main

import (
	"bytes"
	"sync"
)

var (
	avBlockPool sync.Pool
)

// 表示一块音/视频数据
type AVBlock struct {
	Type      byte         // 音/视频
	Timestamp uint32       // 时间戳
	Data      bytes.Buffer // 数据
	Next      *AVBlock     // 下一块数据
}

// 表示一块连续的音/视频数据块缓存
type AVStream struct {
	dataMutex sync.RWMutex
	Valid     bool
	Data      *AVBlock
}
