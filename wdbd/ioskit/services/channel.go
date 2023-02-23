package services

import dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"

type Channel struct {
	r     *RemoteServer
	value int
}

func (c Channel) Call(selector string) (*ChannelFragmenter, error) {
	auxiliary := dtx.NewPrimitiveDictionary()
	err := c.r.SendMessage(c.value, selector, auxiliary, true)
	if err != nil {
		return nil, err
	}

	return c.r.RecvMessage(ChannelCode(c.value))
}
