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
	readChunkSize         uint32                   // 接收消息的一个chunk的大小
	writeChunkSize        uint32                   // 发送消息的一个chunk的大小
	readMessage           map[uint32]*rtmp.Message // 所有正在读取的消息
	lastMessage           rtmp.Message             // 上一个完整的消息
	syncChunkHeader       rtmp.ChunkHeader         // 同步消息使用
	syncMessageBuffer     bytes.Buffer             // 同步消息缓存，以便一次发出多条
	ackMessageData        [4]byte                  // ack消息的缓存
	acknowledgement       uint32                   // 收到的消息数据的大小
	windowAcknowledgeSize uint32                   // control.set_acknowledge_size
	bandWidth             uint32                   // control.set_bandWidth.bandwidth
	bandWidthLimit        byte                     // control.set_bandWidth.limit
	connectUrl            *url.URL                 // command.connect.tcUrl
	streamID              uint32                   // command.createStream
	publishStream         *Stream                  // command.publish
	receiveVideo          bool                     // command.receiveVideo
	receiveAudio          bool                     // command.receiveAudio
	playPause             bool                     // command.pause
	playChan              chan *StreamData         // 接收publish的数据
}

// 循环读取并处理消息
func (c *Conn) readLoop() (err error) {
	defer func() {
		for _, v := range c.readMessage {
			rtmp.PutMessage(v)
		}
	}()
	var n uint32
	var ok bool
	var msg *rtmp.Message
	var chunk rtmp.ChunkHeader
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
		case 1:
			if chunk.MessageTimestamp >= rtmp.MaxMessageTimestamp {
				msg.Timestamp = chunk.ExtendedTimestamp
			} else {
				msg.Timestamp += chunk.MessageTimestamp
			}
			msg.Length = chunk.MessageLength
			msg.TypeID = chunk.MessageTypeID
		case 2:
			if chunk.MessageTimestamp >= rtmp.MaxMessageTimestamp {
				msg.Timestamp = chunk.ExtendedTimestamp
			} else {
				msg.Timestamp += chunk.MessageTimestamp
			}
		default:
			// 新的消息，那么以上一个消息作为模板
			if !ok {
				msg.TypeID = c.lastMessage.TypeID
				msg.StreamID = c.lastMessage.StreamID
				msg.Length = c.lastMessage.Length
				msg.Timestamp = c.lastMessage.Timestamp
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
				err = c.checkWriteControlMessageAcknowledgement()
				if err != nil {
					return
				}
				// 记录
				c.lastMessage.TypeID = msg.TypeID
				c.lastMessage.StreamID = msg.StreamID
				c.lastMessage.Length = msg.Length
				c.lastMessage.Timestamp = msg.Timestamp
				// 处理
				err = c.handleMessage(msg)
				// 返回
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
		if data.IsVideo {
			if !c.receiveVideo {
				return nil
			}
			chunk.CSID = rtmp.VideoMessage
			chunk.MessageTypeID = rtmp.VideoMessage
		} else {
			if !c.receiveAudio {
				return nil
			}
			chunk.CSID = rtmp.AudioMessage
			chunk.MessageTypeID = rtmp.AudioMessage
		}
		chunk.MessageStreamID = c.streamID
		chunk.MessageLength = uint32(data.Data.Len())
		if data.Timestamp >= rtmp.MaxMessageTimestamp {
			chunk.MessageTimestamp = data.Timestamp
			chunk.ExtendedTimestamp = data.Timestamp
		} else {
			chunk.MessageTimestamp = data.Timestamp
			chunk.ExtendedTimestamp = 0
		}
		rtmp.WriteMessage(&buff, &chunk, c.writeChunkSize, data.Data.Bytes())
		_, err = c.writer.Write(buff.Bytes())
		return err
	}
	for stream.Valid {
		data, ok := <-c.playChan
		if !ok {
			return
		}
		err = play(data)
		PutStreamData(data)
		if err != nil {
			log.Error(err)
			return
		}
	}
}

func (c *Conn) handleMessage(msg *rtmp.Message) error {
	switch msg.TypeID {
	case rtmp.ControlMessageSetBandWidth:
		return c.handleControlMessageSetBandWidth(msg)
	case rtmp.ControlMessageWindowAcknowledgementSize:
		return c.handleControlMessageWindowAcknowledgementSize(msg)
	case rtmp.ControlMessageAcknowledgement:
		return c.handleControlMessageAcknowledgement(msg)
	case rtmp.ControlMessageAbort:
		return c.handleControlMessageAbort(msg)
	case rtmp.ControlMessageSetChunkSize:
		return c.handleControlMessageSetChunkSize(msg)
	case rtmp.UserControlMessage:
		if msg.Data.Len() < 2 {
			return fmt.Errorf("user control message invalid length <%d>", msg.Data.Len())
		}
		p := msg.Data.Bytes()
		event := binary.BigEndian.Uint16(p)
		log.Debug(rtmp.UserControlMessageString(event))
		return nil
	case rtmp.CommandMessageAMF0:
		amf, err := rtmp.ReadAMF(&msg.Data)
		if err != nil {
			return err
		}
		if name, ok := amf.(string); ok {
			log.Printf(log.DebugLevel, 0, "command message '%s'", name)
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
			case "onMetaData":
				return c.handleCommandMessageMetaData(msg)
			default:
				return nil
			}
		}
		return fmt.Errorf("command message invalid 'name' data type <%s>", reflect.TypeOf(amf).Kind().String())
	case rtmp.AudioMessage:
		return c.handleAudioMessage(msg)
	case rtmp.VideoMessage:
		return c.handleVideoMessage(msg)
	default:
		log.Printf(log.DebugLevel, 0, "message type <%d>", msg.TypeID)
		return nil
	}
}

func (c *Conn) handleVideoMessage(msg *rtmp.Message) (err error) {
	// log.Debug("video message")
	c.publishStream.AddData(true, msg)
	return
}

func (c *Conn) handleAudioMessage(msg *rtmp.Message) (err error) {
	// log.Debug("audio message")
	c.publishStream.AddData(true, msg)
	return
}

func (c *Conn) handleCommandMessageMetaData(msg *rtmp.Message) (err error) {
	var amf interface{}
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	metaData, ok := amf.(map[string]interface{})
	if !ok {
		return fmt.Errorf("command message.'onMetaData' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	if c.publishStream != nil {
		c.publishStream.metaData.Reset()
		rtmp.WriteAMF(&c.publishStream.metaData, "onMetaData")
		rtmp.WriteAMF(&c.publishStream.metaData, metaData)
	}
	return nil
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
	if _type != "live" {
		// 只支持直播类型的推流
		msg.Data.Reset()
		rtmp.WriteAMF(&msg.Data, "onStatus")
		rtmp.WriteAMF(&msg.Data, transactionID)
		rtmp.WriteAMF(&msg.Data, nil)
		rtmp.WriteAMF(&msg.Data, map[string]interface{}{
			"level":       "error",
			"code":        "NetStream.Publish.Error",
			"description": "server only support live",
		})
	} else {
		c.publishStream, ok = c.server.AddPublishStream(c.connectUrl.Path, c.server.Timestamp)
		if !ok {
			// 已经有相同的流
			msg.Data.Reset()
			rtmp.WriteAMF(&msg.Data, "onStatus")
			rtmp.WriteAMF(&msg.Data, transactionID)
			rtmp.WriteAMF(&msg.Data, nil)
			rtmp.WriteAMF(&msg.Data, map[string]interface{}{
				"level":       "error",
				"code":        "NetStream.Publish.Error",
				"description": "other stream is publishing",
			})
		} else {
			msg.Data.Reset()
			msg.WriteBigEndianUint16(rtmp.UserControlMessageStreamBegin)
			msg.WriteBigEndianUint32(c.streamID)
			c.InitControlMessage(rtmp.UserControlMessage, msg.Data.Bytes())
			msg.Data.Reset()
			rtmp.WriteAMF(&msg.Data, "onStatus")
			rtmp.WriteAMF(&msg.Data, transactionID)
			rtmp.WriteAMF(&msg.Data, nil)
			rtmp.WriteAMF(&msg.Data, map[string]interface{}{
				"level": "status",
				"code":  "NetStream.Publish.Start",
			})
		}
	}
	c.InitCommandMessage(msg.Data.Bytes())
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

func (c *Conn) handleCommandMessageReleaseStream(msg *rtmp.Message) (err error) {
	return
}

func (c *Conn) handleCommandMessageCloseStream(msg *rtmp.Message) (err error) {
	return
}

func (c *Conn) handleCommandMessageDeleteStream(msg *rtmp.Message) (err error) {
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
	rtmp.WriteAMF(&msg.Data, "_result")
	rtmp.WriteAMF(&msg.Data, transactionID)
	rtmp.WriteAMF(&msg.Data, nil)
	rtmp.WriteAMF(&msg.Data, c.streamID)
	c.InitCommandMessage(msg.Data.Bytes())
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
	if stream == nil {
		msg.Data.Reset()
		rtmp.WriteAMF(&msg.Data, "onStatus")
		rtmp.WriteAMF(&msg.Data, transactionID)
		rtmp.WriteAMF(&msg.Data, nil)
		rtmp.WriteAMF(&msg.Data, map[string]interface{}{
			"level": "error",
			"Code":  "NetStream.Play.StreamNotFound",
		})
		c.InitCommandMessage(msg.Data.Bytes())
		_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
		return
	}
	// 响应"Control Message Set Chunk Size"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint32(c.server.ChunkSize)
	c.InitControlMessage(rtmp.ControlMessageSetChunkSize, msg.Data.Bytes())
	// 响应"User Control Message Stream Begin"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint16(rtmp.UserControlMessageStreamBegin)
	msg.WriteBigEndianUint32(c.streamID)
	c.InitControlMessage(rtmp.UserControlMessage, msg.Data.Bytes())
	// 响应"Command Message onStatus"消息
	msg.Data.Reset()
	rtmp.WriteAMF(&msg.Data, "onStatus")
	rtmp.WriteAMF(&msg.Data, transactionID)
	rtmp.WriteAMF(&msg.Data, nil)
	rtmp.WriteAMF(&msg.Data, map[string]interface{}{
		"level": "status",
		"Code":  "NetStream.Play.Start",
	})
	c.InitCommandMessage(msg.Data.Bytes())
	// 响应"Command Message onMetaData"消息
	c.syncMessageBuffer.Write(stream.metaData.Bytes())
	_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	if err != nil {
		return
	}
	// play
	c.playChan = make(chan *StreamData, 1)
	stream.AddPlayConn(c)
	go c.playLoop(stream)
	return
}

func (c *Conn) handleCommandMessageCall(msg *rtmp.Message) (err error) {
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
	msg.WriteBigEndianUint32(c.server.WindowAcknowledgeSize)
	c.InitControlMessage(rtmp.ControlMessageWindowAcknowledgementSize, msg.Data.Bytes())
	// 响应"Control Message Set BandWidth"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint32(c.server.BandWidth)
	msg.Data.WriteByte(c.server.BandWidthLimit)
	c.InitControlMessage(rtmp.ControlMessageSetBandWidth, msg.Data.Bytes())
	// 响应"Command Message _result"消息
	msg.Data.Reset()
	rtmp.WriteAMF(&msg.Data, "_result")
	rtmp.WriteAMF(&msg.Data, transactionID)
	rtmp.WriteAMF(&msg.Data, map[string]interface{}{
		"fmsVer": FMSVer,
	})
	rtmp.WriteAMF(&msg.Data, map[string]interface{}{
		"level":          "status",
		"code":           "NetConnection.Connect.Success",
		"objectEncoding": 0,
	})
	c.InitCommandMessage(msg.Data.Bytes())
	if err != nil {
		return
	}
	_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	return
}

func (c *Conn) InitControlMessage(typeID uint8, data []byte) {
	c.syncChunkHeader.MessageTypeID = typeID
	c.syncChunkHeader.MessageStreamID = rtmp.ControlCommandMessageStreamID
	c.syncChunkHeader.MessageTimestamp = 0
	c.syncChunkHeader.ExtendedTimestamp = 0
	c.syncChunkHeader.CSID = rtmp.ControlMessageChunkStreamID
	c.syncChunkHeader.MessageLength = uint32(len(data))
	rtmp.WriteMessage(&c.syncMessageBuffer, &c.syncChunkHeader, c.writeChunkSize, data)
}

func (c *Conn) InitCommandMessage(data []byte) {
	c.syncChunkHeader.MessageTypeID = rtmp.CommandMessageAMF0
	c.syncChunkHeader.MessageStreamID = rtmp.ControlCommandMessageStreamID
	c.syncChunkHeader.MessageTimestamp = 0
	c.syncChunkHeader.ExtendedTimestamp = 0
	c.syncChunkHeader.CSID = rtmp.CommandMessageChunkStreamID
	c.syncChunkHeader.MessageLength = uint32(len(data))
	rtmp.WriteMessage(&c.syncMessageBuffer, &c.syncChunkHeader, c.writeChunkSize, data)
}

// 检查是否需要发送ControlMessageAcknowledgement消息
func (c *Conn) checkWriteControlMessageAcknowledgement() (err error) {
	if c.windowAcknowledgeSize <= c.acknowledgement {
		binary.BigEndian.PutUint32(c.ackMessageData[:], c.acknowledgement)
		c.acknowledgement = 0
		c.InitControlMessage(rtmp.ControlMessageAcknowledgement, c.ackMessageData[:])
		_, err = c.writer.Write(c.syncMessageBuffer.Bytes())
	}
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
