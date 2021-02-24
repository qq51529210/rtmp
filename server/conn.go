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
)

func init() {
	messagePool.New = func() interface{} {
		return &rtmp.Message{}
	}
}

type Conn struct {
	msg           rtmp.MessageReader
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

func (c *Conn) play(stream *Stream) {
	defer stream.RemovePlayConn(c)
	var err error
	var chunk rtmp.ChunkHeader
	chunk.MessageStreamID = c.streamID
	for stream.Valid {
		data, ok := <-c.playChan
		if !ok {
			return
		}
		if data.IsVideo {
			chunk.MessageTypeID = rtmp.VideoMessage
		} else {
			chunk.MessageTypeID = rtmp.AudioMessage
		}
		chunk.MessageTimestamp = data.Timestamp
		if chunk.MessageTimestamp >= rtmp.MaxMessageTimestamp {
			chunk.MessageTimestamp = rtmp.MaxMessageTimestamp
		}
		err = rtmp.WriteMessage(c.conn, &chunk, c.msg.RemoteChunkSize, data.Data)
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
	var commandObject map[string]interface{}
	commandObject, ok = amf.(map[string]interface{})
	if !ok {
		return fmt.Errorf("command message.'connect'.'command object' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
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
		return fmt.Errorf("command message.'pause'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
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
		return fmt.Errorf("command message.'seek'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// milliSeconds
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok = amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'seek'.'milliSeconds' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	return
}

func (c *Conn) handleCommandMessagePublish(msg *rtmp.Message) (err error) {
	var amf interface{}
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	// transaction id
	tid, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'publish'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
		msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
			"level":       "error",
			"code":        "NetStream.Publish.Error",
			"description": "server only support live",
		})
	} else {
		c.publishStream, ok = c.server.AddPublishStream(c.connectUrl.Path, c.server.Timestamp)
		if !ok {
			// 已经有相同的流
			msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
				"level":       "error",
				"code":        "NetStream.Publish.Error",
				"description": "other stream is publishing",
			})
		}
		msg.InitUserControlMessageStreamBegin(c.streamID)
		err = msg.Write(c.conn, c.RemoteChunkSize)
		if err != nil {
			return
		}
		msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
			"level": "status",
			"code":  "NetStream.Publish.Start",
		})
	}
	err = msg.Write(c.conn, c.RemoteChunkSize)
	if err != nil {
		return
	}
	return c.conn.Flush()
}

func (c *Conn) handleCommandMessageFCPublish(msg *rtmp.Message) (err error) {
	var amf interface{}
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return err
	}
	// transaction id
	tid, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'FCPublish'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
		return fmt.Errorf("command message.'FCPublish'.'publishing name' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	c.publishStream, ok = c.server.AddPublishStream(c.connectUrl.Path, c.server.Timestamp)
	if !ok {
		// 已经有相同的流
		msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
			"level":       "error",
			"code":        "NetStream.Publish.Error",
			"description": "other stream is publishing",
		})
	}
	msg.InitUserControlMessageStreamBegin(c.streamID)
	err = msg.Write(c.conn, c.RemoteChunkSize)
	if err != nil {
		return
	}
	msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
		"level": "status",
		"code":  "NetStream.Publish.Start",
	})
	err = msg.Write(c.conn, c.RemoteChunkSize)
	if err != nil {
		return
	}
	return c.conn.Flush()
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
		return fmt.Errorf("command message.'receiveVideo'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
		return fmt.Errorf("command message.'receiveAudio'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
	var amf interface{}
	// transaction id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'deleteStream'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// stream id
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok = amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'deleteStream'.'streamID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	return
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
		return fmt.Errorf("command message.'play2'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
	tid, ok := amf.(float64)
	if !ok {
		return fmt.Errorf("command message.'play'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// command object is nil
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	// stream name
	amf, err = rtmp.ReadAMF(&msg.Data)
	if err != nil {
		return
	}
	_, ok = amf.(string)
	if !ok {
		return fmt.Errorf("command message.'play'.'streamName' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	// start
	if msg.Data.Len() > 0 {
		amf, err = rtmp.ReadAMF(&msg.Data)
		if err != nil {
			return
		}
		_, ok = amf.(float64)
		if !ok {
			return fmt.Errorf("command message.'play'.'start' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
		}
	}
	// duration
	if msg.Data.Len() > 0 {
		amf, err = rtmp.ReadAMF(&msg.Data)
		if err != nil {
			return
		}
		_, ok = amf.(float64)
		if !ok {
			return fmt.Errorf("command message.'play'.'duration' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
		}
	}
	// reset
	reset := false
	if msg.Data.Len() > 0 {
		amf, err = rtmp.ReadAMF(&msg.Data)
		if err != nil {
			return
		}
		reset, ok = amf.(bool)
		_, ok = amf.(float64)
		if !ok {
			return fmt.Errorf("command message.'play'.'reset' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
		}
	}
	stream := c.server.GetPublishStream(c.connectUrl.Path)
	if stream == nil {
		msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
			"level": "error",
			"Code":  "NetStream.Play.StreamNotFound",
		})
		err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
		if err != nil {
			return
		}
		return c.conn.Flush()
	}
	//
	msg.InitControlMessageSetChunkSize(c.RemoteChunkSize)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	msg.InitUserControlMessageStreamIsRecorded(c.streamID)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	msg.InitUserControlMessageStreamBegin(c.streamID)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	if reset {
		msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
			"level": "status",
			"Code":  "NetStream.Play.Reset",
		})
		err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
		if err != nil {
			return
		}
	}
	msg.InitCommandMessage("onStatus", tid, nil, map[string]interface{}{
		"level": "status",
		"Code":  "NetStream.Play.Start",
	})
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	err = c.conn.Flush()
	if err != nil {
		return
	}
	c.playChan = make(chan *StreamData, 1)
	stream.AddPlayConn(c)
	go c.play(stream)
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
		return fmt.Errorf("command message.'createStream'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
	}
	msg.InitCommandMessage("_result", transactionID, nil, c.streamID)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		messagePool.Put(err)
		return
	}
	c.streamID++
	return c.conn.Flush()
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
		return fmt.Errorf("command message.'connect'.'transactionID' invalid data type <%s>", reflect.TypeOf(amf).Kind().String())
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
	// optional user arguments
	//
	msg.InitControlMessageWindowAcknowledgementSize(c.server.AckSize)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	msg.InitControlMessageSetBandWidth(c.server.BandWidth, c.server.BandWidthLimit)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	msg.InitUserControlMessageStreamBegin(c.streamID)
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
	if err != nil {
		return
	}
	msg.InitCommandMessage("_result", transactionID, map[string]interface{}{
		"fmsVer":       "FMS/3",
		"capabilities": 13,
	}, map[string]interface{}{
		"level":          "status",
		"code":           "NetConnection.Connect.Success",
		"objectEncoding": 0,
	})
	err = msg.Write(c.conn, c.MessageReader.RemoteChunkSize)
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
