package xctest

import (
	"errors"
	"fmt"
	"path"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/instruments"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type XctestRunner struct {
	channel    services.Channel
	idechannel services.Channel
	sps        *dvt.DvtSecureSocketProxyService
	device     ios.DeviceEntry
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
	x.config = nskeyedarchiver.NewXCTestConfiguration(x.targetAppBundleName, x.testSessionID, x.targetAppBundleID, x.targetAppPath, testBundleURL)
	configStr, err := nskeyedarchiver.ArchiveXML(x.config)
	if err != nil {
		return err
	}
	err = fsync.SendFile([]byte(configStr), configFilePath)
	if err != nil {
		return err
	}
	return nil
}

func NewXctestRunner(tms *dvt.TestManagerdSecureService, sps *dvt.DvtSecureSocketProxyService) (*XctestRunner, error) {
	const identifier = "dtxproxy:XCTestManager_IDEInterface:XCTestManager_DaemonConnectionInterface"
	log.Infoln("xctest-runner: MakeChannel")
	channel, err := tms.MakeChannel(identifier)
	if err != nil {
		log.Infoln("xctest-runner: ", err)
		return nil, err
	}

	idechannel := tms.GetXcodeIDEChannel()
	log.Infoln("xctest-runner: ", idechannel)
	s := &XctestRunner{
		channel:    channel,
		idechannel: idechannel,
		sps:        sps,
		device:     tms.GetDevice(),
	}
	return s, nil
}

func (t *XctestRunner) Xctest(
	bundleId string, env map[string]interface{}, args []interface{}, killExisting bool) error {
	info := XctestAppInfo{
		BundleID:             "com.wetest.wda-scrcpy.xctrunner",
		TestRunnerBundleID:   "com.wetest.wda-scrcpy.xctrunner",
		XctestConfigFileName: "scrcpy.xctest",
	}

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
	for k, v := range env {
		_env[k] = v
	}

	process, err := p.Launch(bundleId, _env, _args, killExisting, false)
	if err != nil {
		return err
	}
	//TODO: defer process.Close()

	ok, err := t.authorizeTestSessionWithProcessID(process.Pid)
	if err != nil {
		return err
	}
	log.Infof("authorizing test session for pid %d successful %t", process.Pid, ok)

	err = t.startExecutingTestPlanWithProtocolVersion(36)
	if err != nil {
		return err
	}

	return t.idechannel.RecvLoop()
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
	reply, err2 := t.channel.Call(method, nskeyedarchiver.NewNSUUID(uuid), caps)
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

func (t *XctestRunner) startExecutingTestPlanWithProtocolVersion(version uint64) error {
	const method = "_IDE_startExecutingTestPlanWithProtocolVersion:"
	err := t.idechannel.CallAsync(method, version)
	if err != nil {
		log.Errorf("%v: failed:%v", method, err)
		return err
	}

	return nil
}
