// instruments连接建立之后，传输的消息为 DTXMessage
// DTXMessage = (DTXMessageHeader + DTXPayload)
// - DTXMessageHeader 主要用来对数据进行封包传输，以及说明是否需要应答
// - DTXPayload = (DTXPayloadHeader + DTXPayloadBody)
//     - DTXPayloadHeader 中的flags字段规定了 DTXPayloadBody 的数据类型
//     - DTXPayloadBody 可以是任何数据类型 (None, (None, None), List) 都有可能

package services

import (
	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
)

type ChannelCode int
type RemoteServer struct {
	BaseService
	supportedIdentifiers map[string]interface{}
	lastChannelCode      ChannelCode
	curMessage           int
	channelCache         map[string]interface{}
	// channelessages       map[ChannelCode]
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
	}
	err := s.init(device)
	return s, err
}

func (r *RemoteServer) Init() {

}

func (r *RemoteServer) performHandshake() error {
	// https://github.com/doronz88/pymobiledevice3/blob/ecd4b36716837fbae72e1c31474f3ec9d6edeeca/pymobiledevice3/services/remote_server.py#L261
	args := map[string]interface{}{
		"com.apple.private.DTXBlockCompression": uint64(0),
		"com.apple.private.DTXConnection":       uint64(1),
	}
	auxiliary := dtx.NewPrimitiveDictionary()
	auxiliary.AddNsKeyedArchivedObject(args)
	return r.sendMessage(DtxBroadcastChannelId, "_notifyOfPublishedCapabilities:", auxiliary, false)
}

func (r *RemoteServer) sendMessage(channel int, selector string, args dtx.PrimitiveDictionary, expectsReply bool) error {
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

func (r *RemoteServer) recvMessage(channel int) error {
}
