package xctest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/instruments"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type XctestRunner struct {
	channel  services.Channel
	channel2 services.Channel
	sps      *dvt.DvtSecureSocketProxyService
	device   ios.DeviceEntry
	iOS14    bool
}

type XctestAppInfo struct {
	BundleID string

	testrunnerAppPath   string
	testRunnerHomePath  string
	targetAppPath       string
	targetAppBundleName string
	targetAppBundleID   string

	testSessionID             uuid.UUID
	testConfigurationFilePath string
	testBundlePath            string
	config                    nskeyedarchiver.XCTestConfiguration
}

func (x *XctestAppInfo) Setup(device ios.DeviceEntry) error {
	insproxy, err := installationproxy.New(device)
	if err != nil {
		return err
	}
	defer insproxy.Close()
	apps, err := insproxy.BrowseUserApps()
	found := false
	for _, app := range apps {
		if app.CFBundleIdentifier == x.BundleID {
			x.targetAppPath = app.Path
			x.targetAppBundleName = app.CFBundleName
			x.targetAppBundleID = app.CFBundleIdentifier
			x.testrunnerAppPath = app.Path
			x.testRunnerHomePath = app.EnvironmentVariables["HOME"].(string)
			found = true
			break
		}
	}
	if !found {
		return errors.New("xctest app not existed!")
	}

	fsync, err := afc.NewHouseArrestContainerFs(device, x.BundleID)
	if err != nil {
		return err
	}
	defer fsync.Close()

	x.testSessionID = uuid.New()
	configFilePath := "tmp/" + x.testSessionID.String() + ".xctestconfiguration"
	x.testConfigurationFilePath = x.testRunnerHomePath + "/" + configFilePath
	targetName := strings.Split(x.targetAppBundleName, "-")[0]
	x.testBundlePath = x.testrunnerAppPath + "/PlugIns/" + targetName + ".xctest"

	// FIXME: go-ios的神奇实现，config只能被操作一次
	//    config := nskeyedarchiver.NewXCTestConfiguration
	//    nskeyedarchiver.ArchiveXML(config)
	//    nskeyedarchiver.ArchiveBin(config) <-- 这一句必崩溃
	x.config = nskeyedarchiver.NewXCTestConfiguration(x.targetAppBundleName, x.testSessionID, x.targetAppBundleID, x.targetAppPath, x.testBundlePath)
	config := nskeyedarchiver.NewXCTestConfiguration(x.targetAppBundleName, x.testSessionID, x.targetAppBundleID, x.targetAppPath, x.testBundlePath)
	configStr, err := nskeyedarchiver.ArchiveXML(config)
	if err != nil {
		return err
	}
	err = fsync.SendFile([]byte(configStr), configFilePath)
	if err != nil {
		return err
	}
	return nil
}

func NewXctestRunner(tms1 *dvt.TestManagerdSecureService, tms2 *dvt.TestManagerdSecureService, sps *dvt.DvtSecureSocketProxyService) (*XctestRunner, error) {
	const identifier = "dtxproxy:XCTestManager_IDEInterface:XCTestManager_DaemonConnectionInterface"
	channel, err := tms1.MakeChannel(identifier)
	if err != nil {
		log.Errorln("xctest-runner: ", err)
		return nil, err
	}

	channel2, err := tms2.MakeChannel(identifier)
	if err != nil {
		log.Errorln("xctest-runner: ", err)
		return nil, err
	}

	s := &XctestRunner{
		channel:  channel,
		channel2: channel2,
		sps:      sps,
		device:   tms1.GetDevice(),
		iOS14:    tms1.IsSecure(),
	}
	return s, nil
}

func (t *XctestRunner) Xctest(info XctestAppInfo, env map[string]interface{}, args []interface{}, killExisting bool) error {
	var protover uint64
	if t.iOS14 {
		protover = 36
	} else {
		protover = 25
	}

	err := info.Setup(t.device)
	if err != nil {
		return err
	}

	if t.iOS14 {
		_, err = t.initiateControlSessionWithCapabilities()
		if err != nil {
			return err
		}

		localCaps := nskeyedarchiver.XCTCapabilities{CapabilitiesDictionary: map[string]interface{}{
			"XCTIssue capability":     uint64(1),
			"skipped test capability": uint64(1),
			"test timeout capability": uint64(1),
		}}
		_, err = t.initiateSessionWithIdentifierAndCaps(info.testSessionID, localCaps)
		if err != nil {
			return err
		}
	} else {
		// iOS version < 14.0
		t.initiateSessionWithIdentifier(info.testSessionID, protover)
	}

	p, err := instruments.NewProcessControl(t.sps)
	if err != nil {
		return err
	}

	log.Infoln("XCTestBundlePath", info.testBundlePath)
	log.Infoln("XCTestConfigurationFilePath", info.testConfigurationFilePath)
	log.Infoln("XCTestSessionIdentifier", info.testSessionID.String())

	// init args and enviroment vars
	_args := []interface{}{}
	_env := map[string]interface{}{
		"DYLD_INSERT_LIBRARIES":       "/Developer/usr/lib/libMainThreadChecker.dylib",
		"XCTestBundlePath":            info.testBundlePath,
		"XCTestConfigurationFilePath": info.testConfigurationFilePath,
		"XCTestSessionIdentifier":     info.testSessionID.String(),
	}

	if t.iOS14 {
		ios14args := []interface{}{
			"-NSTreatUnknownArgumentsAsOpen", "NO", "-ApplePersistenceIgnoreState", "YES",
		}
		for _, arg := range ios14args {
			_args = append(_args, arg)
		}

		ios14env := map[string]interface{}{
			"CA_ASSERT_MAIN_THREAD_TRANSACTIONS": "0",
			"CA_DEBUG_TRANSACTIONS":              "0",
			"MTC_CRASH_ON_REPORT":                "1",
			"NSUnbufferedIO":                     "YES",
			"OS_ACTIVITY_DT_MODE":                "YES",
			"SQLITE_ENABLE_THREAD_ASSERTIONS":    "1",
		}

		for k, v := range ios14env {
			_env[k] = v
		}
	}

	// merge user's args and envs
	for _, arg := range args {
		_args = append(_args, arg)
	}
	for k, v := range env {
		_env[k] = v
	}

	// launch xctest process
	process, err := p.Launch(info.BundleID, _env, _args, true, false)
	if err != nil {
		return err
	}
	//TODO: defer process.Close()

	// 下面这段代码非必须，未来 p.Wait()可能会被重构成process.Wait()，才更符合通用设计
	go func() {
		log.Infof("== Wait process:%d begin == ", process.Pid)
		p.Wait()
		log.Warnf("== Wait process:%d end:%v==", process.Pid, err)
	}()

	// FIXME: 实验证明，这里的延迟是必须的，否则xctest进程能拉起，但会有很高概率卡主，并打印日志
	// entering wait loop for 600.00s with expectations: `requesting ready for testing
	time.Sleep(time.Second)
	if t.iOS14 {
		ok, err := t.authorizeTestSessionWithProcessID(process.Pid)
		if err != nil {
			return err
		}
		log.Infof("authorizing test session for pid %d successful %t", process.Pid, ok)
	} else {
		// iOS version < 14.0
		ret, err := t.initiateControlSession(process.Pid, protover)
		if err != nil {
			return err
		}
		log.Warnln("initiateControlSession:", ret)
	}
	return t.startExecutingTestPlanWithProtocolVersion(protover, info.config)
}

func (t *XctestRunner) initiateControlSessionWithCapabilities() (caps nskeyedarchiver.XCTCapabilities, err error) {
	const method = "_IDE_initiateControlSessionWithCapabilities:"
	args := nskeyedarchiver.XCTCapabilities{}
	f, err2 := t.channel.Call(method, args)
	if err != nil {
		err = err2
		return
	}
	data, _, err2 := f.Parse()
	// log.Infof("proclist: sel=%v, aux=%#v, exWrr=%v", data, aux, err)
	if err2 != nil {
		err = err2
		return
	}
	log.Infoln("capabilities:", data)
	val, ok := data[0].(nskeyedarchiver.XCTCapabilities)
	if !ok {
		err = fmt.Errorf("%v invalid return type", method)
		return
	}
	caps = val
	return
}

func (t *XctestRunner) initiateSessionWithIdentifierAndCaps(uuid uuid.UUID, in nskeyedarchiver.XCTCapabilities) (caps nskeyedarchiver.XCTCapabilities, err error) {
	const method = "_IDE_initiateSessionWithIdentifier:capabilities:"
	reply, err2 := t.channel2.Call(method, nskeyedarchiver.NewNSUUID(uuid), caps)
	if err2 != nil {
		err = err2
		return
	}
	data, _, err2 := reply.Parse()
	// log.Infof("proclist: sel=%v, aux=%#v, exWrr=%v", data, aux, err)
	if err2 != nil {
		err = err2
		return
	}
	log.Infoln("capabilities:", data)
	val, ok := data[0].(nskeyedarchiver.XCTCapabilities)
	if !ok {
		err = fmt.Errorf("%v invalid return type", method)
		return
	}
	caps = val
	return
}

func (t *XctestRunner) authorizeTestSessionWithProcessID(pid uint64) (bool, error) {
	const method = "_IDE_authorizeTestSessionWithProcessID:"
	f, err := t.channel.Call(method, pid)
	if err != nil {
		log.Errorf("%v: failed:%v", method, err)
		return false, err
	}
	reply, _, err := f.Parse()
	if _, ok := reply[0].(bool); !ok {
		log.Errorf("%v: invalid reply:%v", method, err)
		return false, fmt.Errorf("%v: invalid reply:%v", method, err)
	}
	return reply[0].(bool), nil
}

func (t *XctestRunner) startExecutingTestPlanWithProtocolVersion(version uint64, testConfig nskeyedarchiver.XCTestConfiguration) error {
	handleFragment := func(f services.Fragment) ([]byte, bool) {
		ph, data, aux, err := f.ParseEx()
		log.Infoln("  ", services.LogDtx(f.DTXMessageHeader, ph))
		log.Infoln("    ", data, aux, err)

		ack := f.NeedAck()
		if len(data) == 0 {
			log.Panic("unknown reply")
			return nil, false
		}
		method, ok := data[0].(string)
		if !ok {
			log.Panic("invalid method")
		}

		switch method {
		case "_requestChannelWithCode:identifier:":
			// aux[0].int
		case "_notifyOfPublishedCapabilities:":
		case "_XCT_didBeginExecutingTestPlan":
		case "_XCT_didBeginInitializingForUITesting":
		case "_XCT_testSuite:didStartAt:":
		case "_XCT_testCase:method:willStartActivity:":
		case "_XCT_testCase:method:didFinishActivity:":
		case "_XCT_testCaseDidStartForTestClass:method:":
		case "_XCT_testBundleReadyWithProtocolVersion:minimumVersion:":
		case "_XCT_logDebugMessage:":
		case "_XCT_testRunnerReadyWithCapabilities:":
			ack = false
			payload, _ := nskeyedarchiver.ArchiveBin(testConfig)
			buf, _ := dtx.Encode(
				int(f.Identifier),  // Identifier
				1,                  // ConversationIndex
				int(f.ChannelCode), // ChannelCode
				false,              // ExpectsReply
				services.ResponseWithReturnValueInPayload, // MessageType
				payload,                      // payloadBytes
				dtx.NewPrimitiveDictionary()) // PrimitiveDictionary
			log.Infof("%v --> ack", f.ChannelCode)
			return buf, true
		case "_XCT_didFinishExecutingTestPlan":
		default:
			log.Warningln(method)
		}

		if ack {
			log.Infof("%v --> ack", f.ChannelCode)
			b := services.BuildDtxAck(f.Identifier, f.ConversationIndex, services.ChannelCode(f.ChannelCode))
			return b, true
		}
		return nil, false
	}

	const method = "_IDE_startExecutingTestPlanWithProtocolVersion:"
	var err error
	log.Infof("== RecvLoop: begin ==")
	if t.iOS14 {
		err = t.channel2.Service().MakeChannelWith(0xFFFFFFFF).CallAsync(method, version)
		if err != nil {
			log.Errorf("%v: failed:%v", method, err)
			return err
		}
		err = t.channel2.Service().RecvLoop(handleFragment)
	} else {
		err = t.channel.Service().MakeChannelWith(0xFFFFFFFF).CallAsync(method, version)
		if err != nil {
			log.Errorf("%v: failed:%v", method, err)
			return err
		}
		err = t.channel.Service().RecvLoop(handleFragment)
	}

	log.Errorf("== RecvLoop: end with %v==", err)
	return err
}

// The following is for iOS version < 14.0
func (t *XctestRunner) initiateSessionWithIdentifier(sessionIdentifier uuid.UUID, version uint64) (uint64, error) {
	const method = "_IDE_initiateSessionWithIdentifier:forClient:atPath:protocolVersion:"
	f, err := t.channel.Call(method,
		nskeyedarchiver.NewNSUUID(sessionIdentifier),
		"thephonedoesntcarewhatisendhereitseems",
		"/Applications/Xcode.app",
		version)
	if err != nil {
		log.Errorf("%v: failed:%v", method, err)
		return 0, err
	}
	reply, _, err := f.Parse()
	if _, ok := reply[0].(uint64); !ok {
		log.Errorf("%v: invalid reply:%v", method, err)
		return 0, fmt.Errorf("%v: invalid reply:%v", method, err)
	}
	return reply[0].(uint64), nil
}

func (t *XctestRunner) initiateControlSession(pid uint64, version uint64) (uint64, error) {
	const method = "_IDE_initiateControlSessionForTestProcessID:protocolVersion:"
	f, err := t.channel2.Call(method, pid, version)
	if err != nil {
		log.Errorf("%v: failed:%v", method, err)
		return 0, err
	}
	reply, _, err := f.Parse()
	if _, ok := reply[0].(uint64); !ok {
		log.Errorf("%v: invalid reply:%v", method, err)
		return 0, fmt.Errorf("%v: invalid reply:%v", method, err)
	}
	return reply[0].(uint64), nil
}
