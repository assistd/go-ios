package services

import (
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
)

type Channel struct {
	r     *RemoteServer
	value uint32
}

func BuildChannel(r *RemoteServer, channel uint32) Channel {
	return Channel{r, channel}
}

func (c Channel) Call(selector string, args ...interface{}) (Fragment, error) {
	auxiliary := dtx.NewPrimitiveDictionary()
	for _, arg := range args {
		auxiliary.AddNsKeyedArchivedObject(arg)
	}
	err := c.r.SendMessage(c.value, selector, auxiliary, true)
	if err != nil {
		return Fragment{}, err
	}

	return c.r.RecvChannel(ChannelCode(c.value))
}

func (c Channel) CallAsync(selector string, args ...interface{}) error {
	auxiliary := dtx.NewPrimitiveDictionary()
	for _, arg := range args {
		auxiliary.AddNsKeyedArchivedObject(arg)
	}
	err := c.r.SendMessage(c.value, selector, auxiliary, false)
	if err != nil {
		return err
	}
	return nil
}
