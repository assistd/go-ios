package services

import (
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	log "github.com/sirupsen/logrus"
)

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

func (c Channel) RecvLoop() error {
	for {
		reply, err := c.r.RecvMessage(ChannelCode(c.value))
		if err != nil {
			log.Errorln(err)
			return err
		}

		data, aux, err := reply.Parse()
		if err != nil {
			log.Errorln(err)
			continue
		}

		method := data[0].(string)
		log.Infoln("recevied:", method, aux)

		switch method {
		case "_XCT_testBundleReadyWithProtocolVersion:minimumVersion:":
		case "_XCT_logDebugMessage:":
		case "_XCT_testRunnerReadyWithCapabilities:":
			// TODO??
		case "_XCT_didFinishExecutingTestPlan":
		}
	}
}
