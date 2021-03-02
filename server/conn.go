package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"path"
	"reflect"
	"sync"

	"github.com/qq51529210/log"
	"github.com/qq51529210/rtmp"
)

var (
	messagePool sync.Pool
	FMSVer      = "rtmp server v1.0"
)

func init() {
	messagePool.New = func() interface{} {
		return &rtmp.Message{}
	}
}

type Conn struct {
	server                *Server
	reader                io.Reader
	writer                io.Writer
	readMessage           map[uint32]*rtmp.Message
	readChunkSize         uint32           // 接收消息的一个chunk的大小
	writeChunkSize        uint32           // 发送消息的一个chunk的大小
	syncChunkHeader       rtmp.ChunkHeader // 同步消息使用
	syncMessageBuffer     bytes.Buffer     // 同步消息缓存，以便一次发出多条
	acknowledgement       uint32           // 收到的消息数据的大小
	windowAcknowledgeSize uint32           // 接收消息的值
	bandWidth             uint32           // 接收消息的值
	bandWidthLimit        byte             // 接收消息的值
	connectUrl            *url.URL         // 接收消息的值
	streamID              uint32           // createStream递增
	publishStream         *Stream          // 接收消息的值
	receiveVideo          bool             // 接收消息的值
	receiveAudio          bool             // 接收消息的值
	playPause             bool             // 接收消息的值
	playChan              chan *StreamGOP  // 可以播放的数据
}

// 播放，循环发送音视频数据
func (c *Conn) playLoop(stream *Stream) {
	defer stream.RemovePlayConn(c)
	var err error
	var chunk rtmp.ChunkHeader
	var buff bytes.Buffer
	var play = func(data *StreamData) error {
		if c.playPause {
			return nil
		}
		if data.typeID == rtmp.VideoMessage {
			if !c.receiveVideo {
				return nil
			}
			chunk.CSID = rtmp.VideoMessageChunkStreamID
			chunk.MessageTypeID = rtmp.VideoMessage
		} else {
			if !c.receiveAudio {
				return nil
			}
			chunk.CSID = rtmp.AudioMessageChunkStreamID
			chunk.MessageTypeID = rtmp.AudioMessage
		}
		chunk.MessageStreamID = c.streamID
		chunk.MessageLength = uint32(data.data.Len())
		if data.timestamp >= rtmp.MaxMessageTimestamp {
			chunk.MessageTimestamp = rtmp.MaxMessageTimestamp
			chunk.ExtendedTimestamp = data.timestamp
		} else {
			chunk.MessageTimestamp = data.timestamp
			chunk.ExtendedTimestamp = 0
		}
		buff.Reset()
		rtmp.WriteMessage(&buff, &chunk, c.writeChunkSize, data.data.Bytes())
		_, err = c.writer.Write(buff.Bytes())
		return err
	}
	err = play(stream.avc)
	if err != nil {
		log.Error(err)
		return
	}
	for stream.valid {
		gop, ok := <-c.playChan
		if !ok {
			return
		}
		for ele := gop.data.Front(); ele != nil; ele = ele.Next() {
			data := ele.Value.(*StreamData)
			err = play(data)
			if err != nil {
				log.Error(err)
				gop.Release()
				return
			}
		}
		gop.Release()
	}
}

// 循环读取并处理消息
func (c *Conn) readLoop() (err error) {
	defer func() {
		for _, v := range c.readMessage {
			rtmp.PutMessage(v)
		}
	}()
	c.readMessage = make(map[uint32]*rtmp.Message)
	var lastTypeID uint8
	var lastStreamID uint32
	var lastTimestamp uint32
	var lastLength uint32
	var n uint32
	var ok bool
	var msg *rtmp.Message
	var chunk rtmp.ChunkHeader
	var ackData [4]byte
	for {
		// chunk header
		err = chunk.Read(c.reader)
		if err != nil {
			return
		}
		msg, ok = c.readMessage[chunk.CSID]
		if !ok {
			msg = rtmp.GetMessage()
		}
		switch chunk.FMT {
		case 0:
			if chunk.MessageTimestamp >= rtmp.MaxMessageTimestamp {
				msg.Timestamp = chunk.ExtendedTimestamp
			} else {
				msg.Timestamp = chunk.MessageTimestamp
			}
			msg.Length = chunk.MessageLength
			msg.TypeID = chunk.MessageTypeID
			msg.StreamID = chunk.MessageStreamID
			//
			lastTimestamp = msg.Timestamp
			lastLength = msg.Length
			lastTypeID = msg.TypeID
			lastStreamID = msg.StreamID
		case 1:
			if chunk.MessageTimestamp >= rtmp.MaxMessageTimestamp {
				msg.Timestamp = chunk.ExtendedTimestamp
			} else {
				msg.Timestamp += chunk.MessageTimestamp
			}
			msg.Length = chunk.MessageLength
			msg.TypeID = chunk.MessageTypeID
			//
			lastTimestamp = msg.Timestamp
			lastLength = msg.Length
			lastTypeID = msg.TypeID
			msg.StreamID = lastStreamID
		case 2:
			if chunk.MessageTimestamp >= rtmp.MaxMessageTimestamp {
				msg.Timestamp = chunk.ExtendedTimestamp
			} else {
				msg.Timestamp += chunk.MessageTimestamp
			}
			lastTimestamp = msg.Timestamp
			msg.Length = lastLength
			msg.TypeID = lastTypeID
			msg.StreamID = lastStreamID
		default:
			// 新的消息，那么以上一个消息作为模板
			if !ok {
				msg.Timestamp = lastTimestamp
				msg.TypeID = lastTypeID
				msg.StreamID = lastStreamID
				msg.Length = lastLength
			}
		}
		// chunk data
		if int(msg.Length) > msg.Data.Len() {
			n = msg.Length - uint32(msg.Data.Len())
			if n > c.readChunkSize {
				n = c.readChunkSize
			}
			_, err = io.CopyN(&msg.Data, c.reader, int64(n))
			if err != nil {
				return
			}
			// 读完整个消息了
			if int(msg.Length) <= msg.Data.Len() {
				// ack
				c.acknowledgement += msg.Length
				if c.windowAcknowledgeSize > 0 && c.acknowledgement >= c.windowAcknowledgeSize {
					binary.BigEndian.PutUint32(ackData[:], c.acknowledgement)
					c.acknowledgement = 0
					c.syncMessageBuffer.Reset()
					c.cacheControlMessage(rtmp.ControlMessageAcknowledgement, ackData[:])
					_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
					if err != nil {
						return
					}
				}
				// 处理
				err = c.handleMessage(msg)
				if ok {
					delete(c.readMessage, chunk.CSID)
				}
				rtmp.PutMessage(msg)
				if err != nil {
					return
				}
			} else {
				// 没读完，先缓存
				if !ok {
					c.readMessage[chunk.CSID] = msg
				}
			}
		}
	}
}

func (c *Conn) cacheControlMessage(typeID uint8, data []byte) {
	c.syncChunkHeader.CSID = rtmp.ControlMessageChunkStreamID
	c.syncChunkHeader.MessageTimestamp = 0
	c.syncChunkHeader.MessageLength = uint32(len(data))
	c.syncChunkHeader.MessageTypeID = typeID
	c.syncChunkHeader.MessageStreamID = rtmp.ControlMessageStreamID
	c.syncChunkHeader.ExtendedTimestamp = 0
	rtmp.WriteMessage(&c.syncMessageBuffer, &c.syncChunkHeader, c.writeChunkSize, data)
}

func (c *Conn) cacheCommandMessage(data []byte) {
	c.syncChunkHeader.CSID = rtmp.CommandMessageChunkStreamID
	c.syncChunkHeader.MessageTimestamp = 0
	c.syncChunkHeader.MessageLength = uint32(len(data))
	c.syncChunkHeader.MessageTypeID = rtmp.CommandMessageAMF0
	c.syncChunkHeader.MessageStreamID = rtmp.ControlMessageStreamID
	c.syncChunkHeader.ExtendedTimestamp = 0
	rtmp.WriteMessage(&c.syncMessageBuffer, &c.syncChunkHeader, c.writeChunkSize, data)
}

func (c *Conn) cacheDataMessage(data []byte) {
	c.syncChunkHeader.CSID = rtmp.DataMessageChunkStreamID
	c.syncChunkHeader.MessageTimestamp = 0
	c.syncChunkHeader.MessageLength = uint32(len(data))
	c.syncChunkHeader.MessageTypeID = rtmp.DataMessageAMF0
	c.syncChunkHeader.MessageStreamID = c.streamID
	c.syncChunkHeader.ExtendedTimestamp = 0
	rtmp.WriteMessage(&c.syncMessageBuffer, &c.syncChunkHeader, c.writeChunkSize, data)
}

func (c *Conn) handleMessage(msg *rtmp.Message) error {
	switch msg.TypeID {
	case rtmp.ControlMessageSetBandWidth:
		log.Debug("control message 'set bandwidth'")
		return c.handleControlMessageSetBandWidth(msg)
	case rtmp.ControlMessageWindowAcknowledgementSize:
		log.Debug("control message 'window acknowledgement size'")
		return c.handleControlMessageWindowAcknowledgementSize(msg)
	case rtmp.ControlMessageAcknowledgement:
		log.Debug("control message 'acknowledgement'")
		return c.handleControlMessageAcknowledgement(msg)
	case rtmp.ControlMessageAbort:
		log.Debug("control message 'abort'")
		return c.handleControlMessageAbort(msg)
	case rtmp.ControlMessageSetChunkSize:
		log.Debug("control message 'set chunk size'")
		return c.handleControlMessageSetChunkSize(msg)
	case rtmp.CommandMessageAMF0:
		return c.handleCommandMessage(msg)
	case rtmp.DataMessageAMF0:
		log.Debug("data message amf0")
		return c.handleDataMessage(msg)
	case rtmp.AudioMessage:
		// log.Debug("audio message")
		return c.handleAudioMessage(msg)
	case rtmp.VideoMessage:
		// log.Debug("video message")
		return c.handleVideoMessage(msg)
	default:
		return nil
	}
}

func (c *Conn) handleVideoMessage(msg *rtmp.Message) (err error) {
	// 第一个是sps和pps
	if c.publishStream.avc == nil {
		c.publishStream.avc = StreamDataPool.Get().(*StreamData)
		c.publishStream.avc.typeID = msg.TypeID
		c.publishStream.avc.timestamp = msg.Timestamp
		c.publishStream.avc.data.Reset()
		c.publishStream.avc.data.Write(msg.Data.Bytes())
	} else {
		c.publishStream.AddData(msg)
	}
	return
}

func (c *Conn) handleAudioMessage(msg *rtmp.Message) (err error) {
	c.publishStream.AddData(msg)
	return
}

func (c *Conn) handleCommandMessage(msg *rtmp.Message) error {
	amf, err := rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	if name, ok := amf.(string); ok {
		log.Debug(fmt.Sprintf("command message '%s'", name))
		switch name {
		case "connect":
			return c.handleCommandMessageConnect(msg)
		case "createStream":
			return c.handleCommandMessageCreateStream(msg)
		case "play":
			return c.handleCommandMessagePlay(msg)
		case "receiveAudio":
			return c.handleCommandMessageReceiveAudio(msg)
		case "receiveVideo":
			return c.handleCommandMessageReceiveVideo(msg)
		case "publish":
			return c.handleCommandMessagePublish(msg)
		case "pause":
			return c.handleCommandMessagePause(msg)
		}
		return nil
	}
	return fmt.Errorf("command message invalid 'name' data type <%s>", reflect.TypeOf(amf).Kind().String())
}

// 这里有限定h264和acc
func (c *Conn) handleDataMessage(msg *rtmp.Message) (err error) {
	var amf interface{}
	for msg.Data.Len() > 0 {
		// onMetaData
		amf, err = rtmp.ReadAMF(&msg.Data)
		if err != nil {
			return
		}
		if name, ok := amf.(string); ok && name == "onMetaData" {
			amf, err = rtmp.ReadAMF(&msg.Data)
			if err != nil {
				return
			}
			metaData, ok := amf.(map[string]interface{})
			if !ok {
				return fmt.Errorf("data message.'onMetaData' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
			}
			// h264 avc
			v := metaData["videocodecid"]
			videoCodecID, ok := v.(float64)
			if !ok {
				return fmt.Errorf("data message.'onMetaData'.'videocodecid' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
			}
			if videoCodecID != 7 {
				return fmt.Errorf("data message.'onMetaData'.'videocodecid' <%v> unsupported", videoCodecID)
			}
			// acc
			v = metaData["audiocodecid"]
			audioCodecID, ok := v.(float64)
			if !ok {
				return fmt.Errorf("data message.'onMetaData'.'audiocodecid' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
			}
			if audioCodecID != 10 {
				return fmt.Errorf("data message.'onMetaData'.'audiocodecid' <%v> unsupported", audioCodecID)
			}
			if c.publishStream != nil {
				c.publishStream.metaData.Reset()
				rtmp.WriteAMF(&c.publishStream.metaData, "onMetaData")
				rtmp.WriteAMF(&c.publishStream.metaData, metaData)
			}
		}
	}
	return
}

func (c *Conn) handleCommandMessagePause(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'pause'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// pause/unpause
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	c.playPause, ok = amf.(bool)
	if !ok {
		return fmt.Errorf("command message.'pause'.'pause' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// milliSeconds
	return
}

func (c *Conn) handleCommandMessagePublish(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	transactionID, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'publish'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	// publishing name
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	_, ok = amf.(string)
	if !ok {
		return fmt.Errorf("command message.'publish'.'publishing name' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// publishing type
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	_type, ok := amf.(string)
	if !ok {
		return fmt.Errorf("command message.'publish'.'publishing type' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	c.syncMessageBuffer.Reset()
	msg.Data.Reset()
	if _type != "live" {
		// 只支持直播类型的推流
		rtmp.WriteAMFs(&msg.Data, "onStatus", transactionID, nil, map[string]interface{}{
			"level":       "error",
			"code":        "NetStream.Publish.Error",
			"description": "other stream is publishing",
		})
	} else {
		c.publishStream, ok = c.server.AddPublishStream(c.connectUrl.Path, c.server.Timestamp)
		if !ok {
			// 已经有相同的流
			rtmp.WriteAMFs(&msg.Data, "onStatus", transactionID, nil, map[string]interface{}{
				"level":       "error",
				"code":        "NetStream.Publish.Error",
				"description": "other stream is publishing",
			})
		} else {
			msg.WriteB16(rtmp.UserControlMessageStreamBegin)
			msg.WriteB32(c.streamID)
			c.cacheControlMessage(rtmp.UserControlMessage, msg.Data.Bytes())
			msg.Data.Reset()
			rtmp.WriteAMFs(&msg.Data, "onStatus", transactionID, nil, map[string]interface{}{
				"level": "status",
				"code":  "NetStream.Publish.Start",
			})
		}
	}
	c.cacheCommandMessage(msg.Data.Bytes())
	_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	return
}

func (c *Conn) handleCommandMessageReceiveVideo(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'receiveVideo'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// bool flag
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	c.receiveVideo, ok = amf.(bool)
	if !ok {
		return fmt.Errorf("command message.'receiveVideo'.'bool' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	return
}

func (c *Conn) handleCommandMessageReceiveAudio(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'receiveAudio'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// bool flag
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	c.receiveAudio, ok = amf.(bool)
	if !ok {
		return fmt.Errorf("command message.'receiveAudio'.'bool' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	return
}

func (c *Conn) handleCommandMessageCreateStream(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	transactionID, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'createStream'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	c.streamID++
	c.syncMessageBuffer.Reset()
	msg.Data.Reset()
	rtmp.WriteAMFs(&msg.Data, "_result", transactionID, nil, c.streamID)
	c.cacheCommandMessage(msg.Data.Bytes())
	_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	return
}

func (c *Conn) handleCommandMessagePlay(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	transactionID, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'play'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// stream name
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	var name string
	name, ok = amf.(string)
	if !ok {
		return fmt.Errorf("command message.'play'.'name' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	var stream *Stream
	if name != "" {
		stream = c.server.GetPublishStream(path.Join(c.connectUrl.Path, name))
	} else {
		stream = c.server.GetPublishStream(c.connectUrl.Path)
	}
	c.syncMessageBuffer.Reset()
	msg.Data.Reset()
	if stream == nil {
		rtmp.WriteAMFs(&msg.Data, "onStatus", transactionID, nil, map[string]interface{}{
			"level": "error",
			"Code":  "NetStream.Play.StreamNotFound",
		})
		c.cacheCommandMessage(msg.Data.Bytes())
		_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
		return
	}
	// 响应"User Control Message Stream Begin"消息
	msg.Data.Reset()
	msg.WriteB16(rtmp.UserControlMessageStreamBegin)
	msg.WriteB32(c.streamID)
	c.cacheControlMessage(rtmp.UserControlMessage, msg.Data.Bytes())
	// 响应"Command Message onStatus"消息
	msg.Data.Reset()
	rtmp.WriteAMFs(&msg.Data, "onStatus", transactionID, nil, map[string]interface{}{
		"level": "status",
		"Code":  "NetStream.Play.Start",
	})
	c.cacheCommandMessage(msg.Data.Bytes())
	// 响应"Command Message |RtmpSampleAccess"消息
	msg.Data.Reset()
	rtmp.WriteAMFs(&msg.Data, "|RtmpSampleAccess", transactionID, true, true)
	c.cacheCommandMessage(msg.Data.Bytes())
	// 响应"Command Message onMetaData"消息
	c.cacheDataMessage(stream.metaData.Bytes())
	_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	if err != nil {
		return
	}
	// play routine
	c.playChan = make(chan *StreamGOP, 1)
	stream.AddPlayConn(c)
	go c.playLoop(stream)
	return
}

func (c *Conn) handleCommandMessageConnect(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	transactionID, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'connect'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	var commandObject map[string]interface{}
	commandObject, ok = amf.(map[string]interface{})
	if !ok {
		return fmt.Errorf("command message.'connect'.'command object' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	v := commandObject["tcUrl"]
	tcUrl, ok := v.(string)
	if !ok {
		return fmt.Errorf("command message.'connect'.'command object'.'tcUrl' invalid data type <%s>", reflect.TypeOf(v).Kind().String())
	}
	c.connectUrl, err = url.Parse(tcUrl)
	if err != nil {
		return fmt.Errorf("command message.'connect'.'command object'.'tcUrl' <%s>", err.Error())
	}
	c.syncMessageBuffer.Reset()
	// 响应"Window Acknowledgement Size"消息
	msg.Data.Reset()
	msg.WriteB32(c.server.WindowAcknowledgeSize)
	c.cacheControlMessage(rtmp.ControlMessageWindowAcknowledgementSize, msg.Data.Bytes())
	// 响应"Control Message Set BandWidth"消息
	msg.Data.Reset()
	msg.WriteB32(c.server.BandWidth)
	msg.Data.WriteByte(c.server.BandWidthLimit)
	c.cacheControlMessage(rtmp.ControlMessageSetBandWidth, msg.Data.Bytes())
	// 响应"Control Message Set Chunk Size"消息
	msg.Data.Reset()
	msg.WriteB32(c.server.ChunkSize)
	c.cacheControlMessage(rtmp.ControlMessageSetChunkSize, msg.Data.Bytes())
	c.writeChunkSize = c.server.ChunkSize
	// 响应"Command Message _result"消息
	msg.Data.Reset()
	rtmp.WriteAMFs(&msg.Data, "_result", transactionID, map[string]interface{}{
		"fmsVer": FMSVer,
	}, map[string]interface{}{
		"level":          "status",
		"code":           "NetConnection.Connect.Success",
		"objectEncoding": 0,
	})
	c.cacheCommandMessage(msg.Data.Bytes())
	_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	return
}

func (c *Conn) handleControlMessageSetBandWidth(msg *rtmp.Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 5 {
		return fmt.Errorf("control message 'set bandwidth' invalid length <%d>", len(data))
	}
	c.bandWidth = binary.BigEndian.Uint32(data)
	c.bandWidthLimit = data[4]
	return
}

func (c *Conn) handleControlMessageWindowAcknowledgementSize(msg *rtmp.Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'window acknowledgement size' invalid length <%d>", len(data))
	}
	c.windowAcknowledgeSize = binary.BigEndian.Uint32(data)
	return
}

func (c *Conn) handleControlMessageAcknowledgement(msg *rtmp.Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'acknowledgement' invalid length <%d>", len(data))
	}
	// m.Ack = binary.BigEndian.Uint32(data)
	return
}

func (c *Conn) handleControlMessageAbort(msg *rtmp.Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'abort' invalid length <%d>", len(data))
	}
	csid := binary.BigEndian.Uint32(data)
	// 这个没发回收了
	delete(c.readMessage, csid)
	return
}

func (c *Conn) handleControlMessageSetChunkSize(msg *rtmp.Message) (err error) {
	data := msg.Data.Bytes()
	if len(data) != 4 {
		return fmt.Errorf("control message 'set chunk size' invalid length <%d>", len(data))
	}
	c.readChunkSize = binary.BigEndian.Uint32(msg.Data.Bytes())
	if c.readChunkSize > rtmp.MaxChunkSize {
		c.readChunkSize = rtmp.MaxChunkSize
	}
	return
}
