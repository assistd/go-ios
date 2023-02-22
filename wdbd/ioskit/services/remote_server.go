// instruments连接建立之后，传输的消息为 DTXMessage
// DTXMessage = (DTXMessageHeader + DTXPayload)
// - DTXMessageHeader 主要用来对数据进行封包传输，以及说明是否需要应答
// - DTXPayload = (DTXPayloadHeader + DTXPayloadBody)
//     - DTXPayloadHeader 中的flags字段规定了 DTXPayloadBody 的数据类型
//     - DTXPayloadBody 可以是任何数据类型 (None, (None, None), List) 都有可能

package services

import (
	"errors"

	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
	log "github.com/sirupsen/logrus"
)

type ChannelCode int
type RemoteServer struct {
	BaseService
	supportedIdentifiers map[string]interface{}
	lastChannelCode      ChannelCode
	curMessage           int
	channelCache         map[string]Channel
	channelMessages      map[ChannelCode][]*dtx.Message
}

const (
	DtxBroadcastChannelId    = 0
	DtxInstrumentMessageType = 2
	DtxExpectsReplyMask      = 0x1000
)

func NewRemoteServer(device ios.DeviceEntry, name string) (*RemoteServer, error) {
	s := &RemoteServer{
		BaseService: BaseService{
			Name:        name,
			IsDeveloper: true,
		},
		supportedIdentifiers: make(map[string]interface{}),
		channelCache:         make(map[string]Channel),
		channelMessages:      make(map[ChannelCode][]*dtx.Message),
	}
	err := s.init(device)
	return s, err
}

func (b *RemoteServer) Init(device ios.DeviceEntry) error {
	b.supportedIdentifiers = make(map[string]interface{})
	b.channelCache = make(map[string]Channel)
	b.channelMessages = make(map[ChannelCode][]*dtx.Message)

	if err := b.init(device); err != nil {
		return err
	}
	if err := b.PerformHandshake(); err != nil {
		b.Close()
		return err
	}
	return nil
}

func (r *RemoteServer) PerformHandshake() error {
	// https://github.com/doronz88/pymobiledevice3/blob/ecd4b36716837fbae72e1c31474f3ec9d6edeeca/pymobiledevice3/services/remote_server.py#L261
	args := map[string]interface{}{
		"com.apple.private.DTXBlockCompression": uint64(0),
		"com.apple.private.DTXConnection":       uint64(1),
	}
	auxiliary := dtx.NewPrimitiveDictionary()
	auxiliary.AddNsKeyedArchivedObject(args)
	method := "_notifyOfPublishedCapabilities:"
	err := r.SendMessage(DtxBroadcastChannelId, method, auxiliary, false)
	if err != nil {
		return err
	}
	resp, err := r.RecvMessage(DtxBroadcastChannelId)
	if err != nil {
		return err
	}
	if resp.Payload[0] != method {
		return errors.New("invalid answer")
	}
	if len(resp.Auxiliary.GetArguments()) == 0 {
		return errors.New("invalid answer")
	}

	r.supportedIdentifiers = resp.Auxiliary.GetArguments()[0].(map[string]interface{})
	return nil
}

func (r *RemoteServer) SendMessage(channel int, selector string, args dtx.PrimitiveDictionary, expectsReply bool) error {
	// DTXMessageHeader
	// DTXPayload
	// 	DTXPayloadHeader
	// 	DTXPayloadBody
	//  	args (dtx.PrimitiveDictionary)
	//		selector (nskeyedarchiver, payload)
	r.curMessage += 1
	sel, _ := nskeyedarchiver.ArchiveBin(selector)
	flags := DtxInstrumentMessageType
	if expectsReply {
		flags |= DtxExpectsReplyMask
	}
	bytes, err := dtx.Encode(
		r.curMessage, // Identifier
		0,            // ConversationIndex
		channel,      // ChannelCode
		expectsReply, // ExpectsReply
		flags,        // MessageType
		sel,          // payloadBytes
		args)         // PrimitiveDictionary
	if err != nil {
		panic(err)
	}
	return r.Send(bytes)
}

// makeChannel make a channel
// refer: ios/dtx_codec/connection.go: RequestChannelIdentifier
func (r *RemoteServer) MakeChannel(identifier string) (Channel, error) {
	if _, ok := r.supportedIdentifiers[identifier]; !ok {
		log.Panicf("%v not in %+v", identifier, r.supportedIdentifiers)
	}

	if v, ok := r.channelCache[identifier]; ok {
		return v, nil
	}

	r.lastChannelCode += 1
	code := r.lastChannelCode
	auxiliary := dtx.NewPrimitiveDictionary()
	auxiliary.AddInt32(int(code))
	arch, _ := nskeyedarchiver.ArchiveBin(identifier)
	auxiliary.AddBytes(arch)
	err := r.SendMessage(DtxBroadcastChannelId, "_requestChannelWithCode:identifier:", auxiliary, true)
	if err != nil {
		return Channel{}, err
	}
	// wait reply
	_, err = r.RecvMessage(code)
	if err != nil {
		panic(err)
	}

	chanel := Channel{r, int(code)}
	r.channelCache[identifier] = chanel
	return chanel, nil
}

// func (r *RemoteServer) RecvPlist(channel ChannelCode) (, error) {
// 	m, err := r.recvMessage()
// 	if err != nil {
// 		return nil, err
// 	}

// 	return m.Payload, m.Auxiliary
// }

func (r *RemoteServer) RecvMessage(channel ChannelCode) (*dtx.Message, error) {
	for {
		array, _ := r.channelMessages[channel]
		if len(array) > 0 {
			m := array[0]
			r.channelMessages[channel] = array[1:]
			// not supported compression
			compression := (m.PayloadHeader.Flags & 0xFF00) >> 12
			if compression > 0 {
				panic("compression is not implemented")
			}
			return m, nil
		}
		m, err := dtx.ReadMessage(r.Conn.Reader())
		if err != nil {
			return nil, err
		}

		if m.ConversationIndex == 0 {
			if m.Identifier > r.curMessage {
				log.Warningf("remote-server: dtx header identifier:%d > curMessage:%d", m.Identifier, r.curMessage)
				r.curMessage = m.Identifier
			}
		}

		array = append(array, &m)
		r.channelMessages[ChannelCode(m.ChannelCode)] = array
	}
}
