package xctest

import (
	"errors"
	"fmt"
	"path"
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
	tms      *dvt.TestManagerdSecureService
	device   ios.DeviceEntry
}

type XctestAppInfo struct {
	BundleID             string
	TestRunnerBundleID   string
	XctestConfigFileName string

	testrunnerAppPath   string
	testRunnerHomePath  string
	targetAppPath       string
	targetAppBundleName string
	targetAppBundleID   string

	testSessionID uuid.UUID
	absConfigPath string
	config        nskeyedarchiver.XCTestConfiguration
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
		if app.CFBundleIdentifier == x.TestRunnerBundleID {
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

	fsync, err := afc.NewHouseArrestContainerFs(device, x.TestRunnerBundleID)
	if err != nil {
		return err
	}
	defer fsync.Close()

	x.testSessionID = uuid.New()
	configFilePath := path.Join("tmp", x.testSessionID.String()+".xctestconfiguration")
	x.absConfigPath = path.Join(x.testRunnerHomePath, configFilePath)
	testBundleURL := path.Join(x.testrunnerAppPath, "PlugIns", x.XctestConfigFileName)

	// FIXME: go-ios的神奇实现，config只能被操作一次
	//    config := nskeyedarchiver.NewXCTestConfiguration
	//    nskeyedarchiver.ArchiveXML(config)
	//    nskeyedarchiver.ArchiveBin(config) <-- 这一句必崩溃
	x.config = nskeyedarchiver.NewXCTestConfiguration(x.targetAppBundleName, x.testSessionID, x.targetAppBundleID, x.targetAppPath, testBundleURL)
	config := nskeyedarchiver.NewXCTestConfiguration(x.targetAppBundleName, x.testSessionID, x.targetAppBundleID, x.targetAppPath, testBundleURL)
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

func NewXctestRunner(tms *dvt.TestManagerdSecureService, tms2 *dvt.TestManagerdSecureService, sps *dvt.DvtSecureSocketProxyService) (*XctestRunner, error) {
	const identifier = "dtxproxy:XCTestManager_IDEInterface:XCTestManager_DaemonConnectionInterface"
	log.Infoln("xctest-runner: MakeChannel")
	channel, err := tms.MakeChannel(identifier)
	if err != nil {
		log.Infoln("xctest-runner: ", err)
		return nil, err
	}

	channel2, err := tms2.MakeChannel(identifier)
	if err != nil {
		log.Infoln("xctest-runner: ", err)
		return nil, err
	}

	s := &XctestRunner{
		channel:  channel,
		channel2: channel2,
		sps:      sps,
		tms:      tms2,
		device:   tms.GetDevice(),
	}
	return s, nil
}

func (t *XctestRunner) Xctest(info XctestAppInfo, env map[string]interface{}, args []interface{}, killExisting bool) error {
	err := info.Setup(t.device)
	if err != nil {
		return err
	}

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

	p, err := instruments.NewProcessControl(t.sps)
	if err != nil {
		return err
	}

	// build args
	_args := []interface{}{
		"-NSTreatUnknownArgumentsAsOpen", "NO", "-ApplePersistenceIgnoreState", "YES",
	}
	for _, arg := range args {
		_args = append(_args, arg)
	}

	// build env
	_env := map[string]interface{}{
		"CA_ASSERT_MAIN_THREAD_TRANSACTIONS": "0",
		"CA_DEBUG_TRANSACTIONS":              "0",
		"DYLD_INSERT_LIBRARIES":              "/Developer/usr/lib/libMainThreadChecker.dylib",

		"MTC_CRASH_ON_REPORT":             "1",
		"NSUnbufferedIO":                  "YES",
		"OS_ACTIVITY_DT_MODE":             "YES",
		"SQLITE_ENABLE_THREAD_ASSERTIONS": "1",
		"XCTestBundlePath":                info.testrunnerAppPath + "/PlugIns/" + info.XctestConfigFileName,
		"XCTestConfigurationFilePath":     info.absConfigPath, // info.testRunnerHomePath + /tmp + <session>.xctestconfiguration
		"XCTestSessionIdentifier":         info.testSessionID.String(),
	}

	log.Infoln("XCTestBundlePath", info.testrunnerAppPath+"/PlugIns/"+info.XctestConfigFileName)
	log.Infoln("XCTestConfigurationFilePath", info.absConfigPath)
	log.Infoln("XCTestSessionIdentifier", info.testSessionID.String())

	for k, v := range env {
		_env[k] = v
	}

	process, err := p.Launch(info.BundleID, _env, _args, killExisting, false)
	if err != nil {
		return err
	}
	//TODO: defer process.Close()

	// FIXME: 实验证明，这里的延迟是必须的，否则xctest app能拉起，显示黑屏，但不执行test
	//
	time.Sleep(time.Second)

	ok, err := t.authorizeTestSessionWithProcessID(process.Pid)
	if err != nil {
		return err
	}
	log.Infof("authorizing test session for pid %d successful %t", process.Pid, ok)
	return t.startExecutingTestPlanWithProtocolVersion(36, info.config)
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
	const method = "_IDE_startExecutingTestPlanWithProtocolVersion:"
	err := t.tms.GetXcodeIDEChannel().CallAsync(method, version)
	if err != nil {
		log.Errorf("%v: failed:%v", method, err)
		return err
	}

	log.Infof("== RecvLoop: begin ==")
	err = t.tms.RecvLoop(func(f services.Fragment) {
		ph, data, aux, err := f.ParseEx()
		log.Infoln("  ", services.LogDtx(f.DTXMessageHeader, ph))
		log.Infoln("    ", data, aux, err)

		ack := ph.Flags == services.Ack
		if len(data) == 0 {
			log.Panic("unknown reply")
			return
		}
		method, ok := data[0].(string)
		if !ok {
			log.Panic("invalid method")
			return
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
			if err := t.tms.Conn.Send(buf); err != nil {
				log.Errorln("Ack failed:")
				return
			}
		case "_XCT_didFinishExecutingTestPlan":
		default:
			log.Warningln(method)
		}

		if ack {
			b := services.BuildDtxAck(f.Identifier, f.ConversationIndex, services.ChannelCode(f.ChannelCode))
			if err := t.tms.Conn.Send(b); err != nil {
				log.Errorln("Ack failed:")
				return
			}
		}
	})

	log.Errorf("== RecvLoop: end with %v==", err)
	return err
}
