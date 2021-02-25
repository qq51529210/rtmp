package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net/url"
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
	*rtmp.Conn
	conn          *bufio.ReadWriter
	connectUrl    *url.URL
	streamID      uint32
	publishStream *Stream
	receiveVideo  bool
	receiveAudio  bool
	playPause     bool
	server        *Server
	playChan      chan *StreamData
}

func (c *Conn) playLoop(stream *Stream) {
	defer stream.RemovePlayConn(c)
	var err error
	var chunkHeader rtmp.ChunkHeader
	chunkHeader.MessageStreamID = c.streamID
	for stream.Valid {
		data, ok := <-c.playChan
		if !ok {
			return
		}
		if c.playPause {
			continue
		}
		if data.IsVideo {
			if !c.receiveVideo {
				continue
			}
			chunkHeader.MessageTypeID = rtmp.VideoMessage
		} else {
			if !c.receiveAudio {
				continue
			}
			chunkHeader.MessageTypeID = rtmp.AudioMessage
		}
		if data.Timestamp >= rtmp.MaxMessageTimestamp {
			chunkHeader.MessageTimestamp = rtmp.MaxMessageTimestamp
			chunkHeader.ExtendedTimestamp = data.Timestamp
		} else {
			chunkHeader.MessageTimestamp = data.Timestamp
			chunkHeader.ExtendedTimestamp = 0
		}
		err = c.Conn.WriteMessage(&chunkHeader, data.Data)
		if err != nil {
			log.Error(err)
			break
		}
		err = c.conn.Flush()
		if err != nil {
			log.Error(err)
			break
		}
		PutStreamData(data)
	}
}

func (c *Conn) handleMessage(msg *rtmp.Message) error {
	switch msg.TypeID {
	case rtmp.UserControlMessage:
		if msg.Data.Len() < 2 {
			return fmt.Errorf("user control message invalid length <%d>", msg.Data.Len())
		}
		p := msg.Data.Bytes()
		event := binary.BigEndian.Uint16(p)
		p = p[2:]
		log.Debug(rtmp.UserControlMessageString(event))
		switch event {
		case rtmp.UserControlMessageStreamBegin:
			return c.handleUserControlMessageStreamBegin(p)
		case rtmp.UserControlMessageStreamEOF:
			return c.handleUserControlMessageStreamEOF(p)
		case rtmp.UserControlMessageStreamDry:
			return c.handleUserControlMessageStreamDry(p)
		case rtmp.UserControlMessageSetBufferLength:
			return c.handleUserControlMessageSetBufferLength(p)
		case rtmp.UserControlMessageStreamIsRecorded:
			return c.handleUserControlMessageStreamIsRecorded(p)
		case rtmp.UserControlMessagePingRequest:
			return c.handleUserControlMessagePingRequest(p)
		case rtmp.UserControlMessagePingResponse:
			return c.handleUserControlMessagePingResponse(p)
		default:
			return nil
		}
	case rtmp.CommandMessageAMF0, rtmp.CommandMessageAMF3:
		amf, err := rtmp.ReadAMF(&msg.Data)
		if err != nil {
			return err
		}
		if name, ok := amf.(string); ok {
			log.Printf(log.DebugLevel, 0, "command message '%s'", name)
			switch name {
			case "connect":
				return c.handleCommandMessageConnect(msg)
			case "call":
				return c.handleCommandMessageCall(msg)
			case "createStream":
				return c.handleCommandMessageCreateStream(msg)
			case "play":
				return c.handleCommandMessagePlay(msg)
			case "play2":
				return c.handleCommandMessagePlay2(msg)
			case "deleteStream":
				return c.handleCommandMessageDeleteStream(msg)
			case "closeStream":
				return c.handleCommandMessageCloseStream(msg)
			case "releaseStream":
				return c.handleCommandMessageCloseStream(msg)
			case "receiveAudio":
				return c.handleCommandMessageReceiveAudio(msg)
			case "receiveVideo":
				return c.handleCommandMessageReceiveVideo(msg)
			case "publish":
				return c.handleCommandMessagePublish(msg)
			case "FCPublish":
				return c.handleCommandMessageFCPublish(msg)
			case "seek":
				return c.handleCommandMessageSeek(msg)
			case "pause":
				return c.handleCommandMessagePause(msg)
			case "onMetaData":
				return c.handleCommandMessageMetaData(msg)
			default:
				return nil
			}
		}
		return fmt.Errorf("command message invalid 'name' data type <%s>", reflect.TypeOf(amf).Kind().String())
	case rtmp.DataMessageAMF0, rtmp.DataMessageAMF3:
		return c.handleDataMessage(msg)
	case rtmp.SharedObjectMessageAMF0, rtmp.SharedObjectMessageAMF3:
		return c.handleSharedObjectMessage(msg)
	case rtmp.AudioMessage:
		return c.handleAudioMessage(msg)
	case rtmp.VideoMessage:
		return c.handleVideoMessage(msg)
	case rtmp.AggregateMessage:
		return c.handleAggregateMessage(msg)
	default:
		log.Printf(log.DebugLevel, 0, "message type <%d>", msg.TypeID)
	}
	return nil
}

func (c *Conn) handleAggregateMessage(msg *rtmp.Message) (err error) {
	log.Debug("aggregate message")
	return
}

func (c *Conn) handleSharedObjectMessage(msg *rtmp.Message) (err error) {
	log.Debug("shared object message")
	return
}

func (c *Conn) handleDataMessage(msg *rtmp.Message) (err error) {
	log.Debug("data message")
	return
}

func (c *Conn) handleAudioMessage(msg *rtmp.Message) (err error) {
	log.Debug("audio message")
	c.publishStream.AddData(false, msg.Timestamp, msg.Data.Bytes())
	return
}

func (c *Conn) handleVideoMessage(msg *rtmp.Message) (err error) {
	log.Debug("video message")
	c.publishStream.AddData(true, msg.Timestamp, msg.Data.Bytes())
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
		c.publishStream.metaData = metaData
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

func (c *Conn) handleCommandMessageSeek(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'seek'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
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
	if _type != "live" {
		// 只支持直播类型的推流
		msg.InitAMF("onStatus", transactionID, nil, map[string]interface{}{
			"level":       "error",
			"code":        "NetStream.Publish.Error",
			"description": "server only support live",
		})
	} else {
		c.publishStream, ok = c.server.AddPublishStream(c.connectUrl.Path, c.server.Timestamp)
		if !ok {
			// 已经有相同的流
			msg.InitAMF("onStatus", transactionID, nil, map[string]interface{}{
				"level":       "error",
				"code":        "NetStream.Publish.Error",
				"description": "other stream is publishing",
			})
		} else {
			msg.Data.Reset()
			msg.WriteBigEndianUint16(rtmp.UserControlMessageStreamBegin)
			msg.WriteBigEndianUint32(c.streamID)
			err = c.Conn.WriteControlMessage(rtmp.UserControlMessage, msg.Data.Bytes())
			if err != nil {
				return
			}
			msg.InitAMF("onStatus", transactionID, nil, map[string]interface{}{
				"level": "status",
				"code":  "NetStream.Publish.Start",
			})
		}
	}
	err = c.Conn.WriteCommandMessage(msg.Data.Bytes())
	if err != nil {
		return
	}
	return c.conn.Flush()
}

func (c *Conn) handleCommandMessageFCPublish(msg *rtmp.Message) (err error) {
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
	msg.InitAMF("_result", transactionID, nil, c.streamID)
	err = c.Conn.WriteCommandMessage(msg.Data.Bytes())
	if err != nil {
		return
	}
	return c.conn.Flush()
}

func (c *Conn) handleCommandMessagePlay2(msg *rtmp.Message) (err error) {
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'play2'.'transaction id' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// parameters object
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
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
	stream := c.server.GetPublishStream(c.connectUrl.Path)
	if stream == nil {
		msg.InitAMF("onStatus", transactionID, nil, map[string]interface{}{
			"level": "error",
			"Code":  "NetStream.Play.StreamNotFound",
		})
		err = c.Conn.WriteCommandMessage(msg.Data.Bytes())
		if err != nil {
			return
		}
		return c.conn.Flush()
	}
	// 响应"Control Message Set Chunk Size"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint32(c.server.ChunkSize)
	err = c.Conn.WriteControlMessage(rtmp.ControlMessageSetChunkSize, msg.Data.Bytes())
	if err != nil {
		return
	}
	c.Conn.SetWriteChunkSize(c.server.ChunkSize)
	// 响应"User Control Message Stream Begin"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint16(rtmp.UserControlMessageStreamBegin)
	msg.WriteBigEndianUint32(c.streamID)
	err = c.Conn.WriteControlMessage(rtmp.ControlMessageSetChunkSize, msg.Data.Bytes())
	if err != nil {
		return
	}
	msg.InitAMF("onStatus", transactionID, nil, map[string]interface{}{
		"level": "status",
		"Code":  "NetStream.Play.Start",
	})
	err = c.Conn.WriteCommandMessage(msg.Data.Bytes())
	if err != nil {
		return
	}
	err = c.conn.Flush()
	if err != nil {
		return
	}
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
	// 响应"Window Acknowledgement Size"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint32(c.server.WindowAcknowledgeSize)
	err = c.Conn.WriteControlMessage(rtmp.ControlMessageWindowAcknowledgementSize, msg.Data.Bytes())
	if err != nil {
		return
	}
	// 响应"Control Message Set BandWidth"消息
	msg.Data.Reset()
	msg.WriteBigEndianUint32(c.server.BandWidth)
	msg.Data.WriteByte(c.server.BandWidthLimit)
	err = c.Conn.WriteControlMessage(rtmp.ControlMessageSetBandWidth, msg.Data.Bytes())
	if err != nil {
		return
	}
	// 响应"Command Message _result"消息
	msg.Data.Reset()
	msg.InitAMF("_result", transactionID, map[string]interface{}{
		"fmsVer": FMSVer,
	}, map[string]interface{}{
		"level":          "status",
		"code":           "NetConnection.Connect.Success",
		"objectEncoding": 0,
	})
	err = c.Conn.WriteCommandMessage(msg.Data.Bytes())
	if err != nil {
		return
	}
	return c.conn.Flush()
}

func (c *Conn) handleUserControlMessagePingResponse(data []byte) (err error) {
	return
}

func (c *Conn) handleUserControlMessagePingRequest(data []byte) (err error) {
	return
}

func (c *Conn) handleUserControlMessageStreamIsRecorded(data []byte) (err error) {
	return
}

func (c *Conn) handleUserControlMessageSetBufferLength(data []byte) (err error) {
	return
}

func (c *Conn) handleUserControlMessageStreamDry(data []byte) (err error) {
	return
}

func (c *Conn) handleUserControlMessageStreamEOF(data []byte) (err error) {
	return
}

func (c *Conn) handleUserControlMessageStreamBegin(data []byte) (err error) {
	return
}
