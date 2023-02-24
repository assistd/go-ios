// instruments连接建立之后，传输的消息为 DTXMessage
// DTXMessage = (DTXMessageHeader + DTXPayload)
// - DTXMessageHeader 主要用来对数据进行封包传输，以及说明是否需要应答
// - DTXPayload = (DTXPayloadHeader + DTXPayloadBody)
//     - DTXPayloadHeader 中的flags字段规定了 DTXPayloadBody 的数据类型
//     - DTXPayloadBody 可以是任何数据类型 (None, (None, None), List) 都有可能

package services

import (
	"errors"
	"io"

	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
	log "github.com/sirupsen/logrus"
)

type ChannelCode uint32
type RemoteServer struct {
	BaseService
	supportedIdentifiers map[string]interface{}
	lastChannelCode      ChannelCode
	curMessage           int
	channelCache         map[string]Channel
	channelMessages      map[ChannelCode]*ChannelFragmenter
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
		channelMessages:      make(map[ChannelCode]*ChannelFragmenter),
	}
	err := s.init(device)
	return s, err
}

func (b *RemoteServer) Init(device ios.DeviceEntry) error {
	b.supportedIdentifiers = make(map[string]interface{})
	b.channelCache = make(map[string]Channel)
	b.channelMessages = make(map[ChannelCode]*ChannelFragmenter)

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
	_, aux, err := resp.Parse()
	if err != nil {
		return errors.New("invalid answer")
	}

	r.supportedIdentifiers = aux
	return nil
}

func (r *RemoteServer) SendMessage(channel uint32, selector string, args dtx.PrimitiveDictionary, expectsReply bool) error {
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
		int(channel), // ChannelCode
		expectsReply, // ExpectsReply
		flags,        // MessageType
		sel,          // payloadBytes
		args)         // PrimitiveDictionary
	if err != nil {
		panic(err)
	}
	return r.Conn.Send(bytes)
}

// makeChannel make a channel
// refer: ios/dtx_codec/connection.go: RequestChannelIdentifier
func (r *RemoteServer) MakeChannel(identifier string) (Channel, error) {
	/*
		if _, ok := r.supportedIdentifiers[identifier]; !ok {
			log.Panicf("%v not in %+v", identifier, r.supportedIdentifiers)
		}
	*/

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
	// wait ACK
	_, err = r.RecvMessage(DtxBroadcastChannelId)
	if err != nil {
		panic(err)
	}

	chanel := Channel{r, uint32(code)}
	r.channelCache[identifier] = chanel
	return chanel, nil
}

func (r *RemoteServer) RecvMessage(channel ChannelCode) (*ChannelFragmenter, error) {
	mheader := &DTXMessageHeader{}
	buf := make([]byte, mheader.Length())
	for {
		// TODO: 这里的实现与pymobiledevice3不一样，没有使用队列，是否可能有问题？
		fragmenter, ok := r.channelMessages[channel]
		if ok && fragmenter.IsFull() {
			// not supported compression
			log.Infof("<-channel:%v fulled", channel)
			delete(r.channelMessages, channel)
			return fragmenter, nil
		}

		/*
			m, _ := dtx.ReadMessage(r.Conn.Reader())
			log.Infof("<--%v", m.StringDebug())
			log.Infof("<--pb header: %#v", m.PayloadHeader)
			log.Infof("<--aux header: %#v", m.AuxiliaryHeader)
			os.Exit(1)
		*/

		_, err := io.ReadFull(r.Conn.Reader(), buf)
		if err != nil {
			return nil, err
		}
		mheader.ReadFrom(buf)

		if mheader.ConversationIndex == 0 {
			if int(mheader.Identifier) > r.curMessage {
				log.Warningf("remote-server: dtx header identifier:%d > curMessage:%d", mheader.Identifier, r.curMessage)
				r.curMessage = int(mheader.Identifier)
			}
		}

		fragmenter, ok = r.channelMessages[ChannelCode(mheader.ChannelCode)]
		if !ok {
			fragmenter = &ChannelFragmenter{}
			r.channelMessages[ChannelCode(mheader.ChannelCode)] = fragmenter
		}

		if mheader.FragmentCount > 1 && mheader.FragmentId == 0 {
			// when reading multiple message fragments, the first fragment contains only a message header
			fragmenter.AddFirst(mheader)
			continue
		}

		chunk := make([]byte, mheader.PayloadLength)
		_, err = io.ReadFull(r.Conn.Reader(), chunk)
		if err != nil {
			return nil, err
		}

		log.Infof("<- fragment: %#v, chunk:%v", mheader, len(chunk))
		fragmenter.Add(mheader, chunk)
	}
}
