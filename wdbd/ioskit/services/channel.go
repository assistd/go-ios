package services

import dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"

type Channel struct {
	r     *RemoteServer
	value int
}

func BuildChannel(r *RemoteServer, channel int) Channel {
	return Channel{r, channel}
}

func (c Channel) Call(selector string, args ...interface{}) (*ChannelFragmenter, error) {
	auxiliary := dtx.NewPrimitiveDictionary()
	for _, arg := range args {
		auxiliary.AddNsKeyedArchivedObject(arg)
	}
	err := c.r.SendMessage(c.value, selector, auxiliary, true)
	if err != nil {
		return nil, err
	}

	return c.r.RecvMessage(ChannelCode(c.value))
}

func (c Channel) CallAsync(selector string, args ...interface{}) error {
	auxiliary := dtx.NewPrimitiveDictionary()
	for _, arg := range args {
		auxiliary.AddNsKeyedArchivedObject(arg)
	}
	err := c.r.SendMessage(c.value, selector, auxiliary, true)
	if err != nil {
		return err
	}
	return nil
}
