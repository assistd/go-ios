package instruments

import (
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	log "github.com/sirupsen/logrus"
)

type ProcessControl struct {
	channel services.Channel
}

type Process struct {
	Pid uint64
}

func NewProcessControl(dvt *dvt.DvtSecureSocketProxyService) (*ProcessControl, error) {
	const identifier = "com.apple.instruments.server.services.processcontrol"
	log.Infoln("process-control: MakeChannel")
	channel, err := dvt.MakeChannel(identifier)
	if err != nil {
		log.Infoln("deviceinfo: ", err)
		return nil, err
	}

	log.Infoln("deviceinfo: ", channel)
	s := &ProcessControl{channel}
	return s, nil
}

// Launch a process.
// param bundle_id: Bundle id of the process.
// param list arguments: List of argument to pass to process.
// param kill_existing: Whether to kill an existing instance of this process.
// param start_suspended: Same as WaitForDebugger.
// param environment: Environment variables to pass to process.
// return: PID of created process.
func (d *ProcessControl) Launch(bundleId string, env map[string]interface{}, args []interface{}, killExisting, startSuspended bool) (*Process, error) {
	const path = "/private/"
	options := map[string]interface{}{
		"KillExisting":      bool2int(killExisting),
		"StartSuspendedKey": bool2int(startSuspended),
		// iOS14以下，ActivateSuspended参数配置后，会在后台拉起xctest，否则会出现黑色窗口
		"ActivateSuspended": uint64(1),
	}

	if env == nil {
		env = make(map[string]interface{})
	}
	if args == nil {
		args = make([]interface{}, 0)
	}

	f, err := d.channel.Call("launchSuspendedProcessWithDevicePath:bundleIdentifier:environment:arguments:options:",
		path, bundleId, env, args, options)
	if err != nil {
		// 偶现这里返回EOF
		panic(err)
	}

	data, aux, err := f.Parse()
	// log.Infof("proclist: sel=%v, aux=%#v, exWrr=%v", data, aux, err)
	if err != nil {
		panic(err)
	}

	log.Infof("data:%#v, aux:%#v, err:%v", data, aux, err)
	pid, ok := data[0].(uint64)
	if !ok {
		panic("invalid reply")
	}

	return &Process{
		Pid: pid,
	}, nil
}

func (d *ProcessControl) Wait() error {
	service := d.channel.Service()
	err := service.RecvLoop(func(f services.Fragment) ([]byte, bool) {
		ph, data, aux, err := f.ParseEx()
		log.Infoln("  ### ", services.LogDtx(f.DTXMessageHeader, ph))
		log.Infoln("    ### ", data, aux, err)
		if f.NeedAck() {
			b := services.BuildDtxAck(f.Identifier, f.ConversationIndex, services.ChannelCode(f.ChannelCode))
			return b, true
		}
		return nil, false
	})
	return err
}

func bool2int(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}
