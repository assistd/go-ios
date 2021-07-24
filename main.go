package main

import (
	"encoding/json"
	"fmt"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"os"
	"os/signal"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/accessibility"
	"github.com/danielpaulus/go-ios/ios/debugproxy"
	"github.com/danielpaulus/go-ios/ios/diagnostics"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/notificationproxy"
	"github.com/danielpaulus/go-ios/ios/pcap"
	"github.com/danielpaulus/go-ios/ios/screenshotr"
	syslog "github.com/danielpaulus/go-ios/ios/syslog"
	"github.com/danielpaulus/go-ios/ios/testmanagerd"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
)

//JSONdisabled enables or disables output in JSON format
var JSONdisabled = false

func main() {
	Main()
}

const version = "local-build"

func initLog() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	log.SetLevel(log.InfoLevel)
}


// Main Exports main for testing
func Main() {
	usage := fmt.Sprintf(`go-ios %s

Usage:
  ios listen [options]
  ios list [options] [--details]
  ios info [--domain=<domain>] [--key=<key>] [options]
  ios image list [options]
  ios image mount [--path=<imagepath>] [options]
  ios image auto [--basedir=<where_dev_images_are_stored>] [options]
  ios syslog [options]
  ios screenshot [options] [--output=<outfile>]
  ios devicename [options] 
  ios date [options]
  ios lang [--setlocale=<locale>] [--setlang=<newlang>] [options]
  ios diagnostics list [options]
  ios pair [options]
  ios ps [options]
  ios forward [options] <hostPort> <targetPort>
  ios dproxy [--binary]
  ios readpair [options]
  ios pcap [options] [--pid=<processID>] [--process=<processName>]
  ios install --path=<ipaOrAppFolder> [options]
  ios apps [--system] [options]
  ios launch <bundleID> [options]
  ios runtest <bundleID> [options]
  ios runwda [--bundleid=<bundleid>] [--testrunnerbundleid=<testbundleid>] [--xctestconfig=<xctestconfig>] [options]
  ios ax [options]
  ios reboot [options]
  ios -h | --help
  ios --version | version [options]

Options:
  -v --verbose   Enable Debug Logging.
  -t --trace     Enable Trace Logging (dump every message).
  --nojson       Disable JSON output (default).
  -h --help      Show this screen.
  --udid=<udid>  UDID of the device.

The commands work as following:
	The default output of all commands is JSON. Should you prefer human readable outout, specify the --nojson option with your command. 
	By default, the first device found will be used for a command unless you specify a --udid=some_udid switch.
	Specify -v for debug logging and -t for dumping every message.

   ios listen [options]                                               Keeps a persistent connection open and notifies about newly connected or disconnected devices.
   ios list [options] [--details]                                     Prints a list of all connected device's udids. If --details is specified, it includes version, name and model of each device.
   ios info [options]                                                 Prints a dump of Lockdown getValues.
   ios image list [options]                                           List currently mounted developers images' signatures
   ios image mount [--path=<imagepath>] [options]                     Mount a image from <imagepath>
   ios image auto [--basedir=<where_dev_images_are_stored>] [options] Automatically download correct dev image from the internets and mount it. You can specify a dir where images should be cached. The default is the current dir. 
   ios syslog [options]                                               Prints a device's log output
   ios screenshot [options] [--output=<outfile>]                      Takes a screenshot and writes it to the current dir or to <outfile>
   ios devicename [options]                                           Prints the devicename
   ios date [options]                                                 Prints the device date
   ios lang [--setlocale=<locale>] [--setlang=<newlang>] [options]    Sets or gets the Device language
   ios diagnostics list [options]                                     List diagnostic infos
   ios pair [options]                                                 Pairs the device without a dialog for supervised devices
   ios ps [options]                                                   Dumps a list of running processes on the device
   ios forward [options] <hostPort> <targetPort>                      Similar to iproxy, forward a TCP connection to the device.
   ios dproxy [--binary]                                              Starts the reverse engineering proxy server. 
   >                                                                  It dumps every communication in plain text so it can be implemented easily. 
   >                                                                  Use "sudo launchctl unload -w /Library/Apple/System/Library/LaunchDaemons/com.apple.usbmuxd.plist"
   >                                                                  to stop usbmuxd and load to start it again should the proxy mess up things.
   >                                                                  The --binary flag will dump everything in raw binary without any decoding. 
   ios readpair                                                       Dump detailed information about the pairrecord for a device.                                              Starts a pcap dump of network traffic
   ios install --path=<ipaOrAppFolder> [options]                      Specify a .app folder or an installable ipa file that will be installed.  
   ios pcap [options] [--pid=<processID>] [--process=<processName>]   Starts a pcap dump of network traffic, use --pid or --process to filter specific processes.
   ios apps [--system]                                                Retrieves a list of installed applications. --system prints out preinstalled system apps.
   ios launch <bundleID>                                              Launch app with the bundleID on the device. Get your bundle ID from the apps command.
   ios runtest <bundleID>                                             Run a XCUITest. 
   ios runwda [options]                                               Start WebDriverAgent
   ios ax [options]                                                   Access accessibility inspector features. 
   ios reboot [options]                                               Reboot the given device
   ios -h | --help                                                    Prints this screen.
   ios --version | version [options]                                  Prints the version

  `, version)
	arguments, err := docopt.ParseDoc(usage)
	exitIfError("failed parsing args", err)
	disableJSON, _ := arguments.Bool("--nojson")
	if disableJSON {
		JSONdisabled = true
	} else {
		log.SetFormatter(&log.JSONFormatter{})
	}

	initLog()

	traceLevelEnabled, _ := arguments.Bool("--trace")
	if traceLevelEnabled {
		log.Info("Set Trace mode")
		log.SetLevel(log.TraceLevel)
	} else {

		verboseLoggingEnabledLong, _ := arguments.Bool("--verbose")

		if verboseLoggingEnabledLong {
			log.Info("Set Debug mode")
			log.SetLevel(log.DebugLevel)
		}
	}
	//log.SetReportCaller(true)
	log.Debug(arguments)

	shouldPrintVersionNoDashes, _ := arguments.Bool("version")
	shouldPrintVersion, _ := arguments.Bool("--version")
	if shouldPrintVersionNoDashes || shouldPrintVersion {
		printVersion()
		return
	}

	b, _ := arguments.Bool("listen")
	if b {
		startListening()
		return
	}

	b, _ = arguments.Bool("list")
	diagnosticsCommand, _ := arguments.Bool("diagnostics")
	imageCommand, _ := arguments.Bool("image")
	if b && !diagnosticsCommand && !imageCommand {
		b, _ = arguments.Bool("--details")
		printDeviceList(b)
		return
	}

	udid, _ := arguments.String("--udid")
	device, err := ios.GetDevice(udid)
	exitIfError("error getting devicelist", err)

	b, _ = arguments.Bool("pcap")
	if b {
		p, _ := arguments.String("--process")
		i, _ := arguments.Int("--pid")
		pcap.Pid = int32(i)
		pcap.ProcName = p
		err := pcap.Start(device)
		if err != nil {
			exitIfError("pcap failed", err)
		}
		return
	}

	b, _ = arguments.Bool("ps")
	if b {
		processList(device)
		return
	}

	b, _ = arguments.Bool("install")
	if b {
		path, _ := arguments.String("--path")
		installApp(device, path)
		return
	}

	b, _ = arguments.Bool("image")
	if b {
		list, _ := arguments.Bool("list")
		if list {
			listMountedImages(device)
		}
		mount, _ := arguments.Bool("mount")
		if mount {
			path, _ := arguments.String("--path")
			mountImage(device, path)
		}
		auto, _ := arguments.Bool("auto")
		if auto {
			basedir, _ := arguments.String("--basedir")
			if basedir == "" {
				basedir = "."
			}
			fixDevImage(device, basedir)
		}
		return
	}

	b, _ = arguments.Bool("lang")
	if b {
		locale, _ := arguments.String("--setlocale")
		newlang, _ := arguments.String("--setlang")
		log.Debugf("lang --setlocale:%s --setlang:%s", locale, newlang)
		language(device, locale, newlang)
		return
	}

	b, _ = arguments.Bool("dproxy")
	if b {
		log.SetFormatter(&log.TextFormatter{})
		//log.SetLevel(log.DebugLevel)
		binaryMode, _ := arguments.Bool("--binary")
		startDebugProxy(device, binaryMode)
		return
	}

	b, _ = arguments.Bool("info")
	if b {
		domain, _ := arguments.String("--domain")
		key, _ := arguments.String("--key")
		printDeviceInfo(device, domain, key)
		return
	}

	b, _ = arguments.Bool("syslog")
	if b {
		runSyslog(device)
		return
	}

	b, _ = arguments.Bool("screenshot")
	if b {
		path, _ := arguments.String("--output")
		saveScreenshot(device, path)
		return
	}
	b, _ = arguments.Bool("devicename")
	if b {
		printDeviceName(device)
		return
	}

	b, _ = arguments.Bool("apps")

	if b {
		system, _ := arguments.Bool("--system")
		printInstalledApps(device, system)
		return
	}

	b, _ = arguments.Bool("date")
	if b {
		printDeviceDate(device)
		return
	}
	b, _ = arguments.Bool("diagnostics")
	if b {
		printDiagnostics(device)
		return
	}

	b, _ = arguments.Bool("pair")
	if b {
		pairDevice(device)
		return
	}

	b, _ = arguments.Bool("readpair")
	if b {
		readPair(device)
		return
	}

	b, _ = arguments.Bool("forward")
	if b {
		hostPort, _ := arguments.Int("<hostPort>")
		targetPort, _ := arguments.Int("<targetPort>")
		startForwarding(device, hostPort, targetPort)
		return
	}

	b, _ = arguments.Bool("launch")
	if b {
		bundleID, _ := arguments.String("<bundleID>")
		if bundleID == "" {
			log.Fatal("please provide a bundleID")
		}
		pControl, err := instruments.NewProcessControl(device)
		exitIfError("processcontrol failed", err)

		pid, err := pControl.LaunchApp(bundleID)
		exitIfError("launch app command failed", err)

		log.WithFields(log.Fields{"pid": pid}).Info("Process launched")
	}

	b, _ = arguments.Bool("runtest")
	if b {
		bundleID, _ := arguments.String("<bundleID>")
		err := testmanagerd.RunXCUITest(bundleID, device)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Info("Failed running Xcuitest")
		}
		return
	}

	b, _ = arguments.Bool("runwda")
	if b {

		bundleID, _ := arguments.String("--bundleid")
		testbundleID, _ := arguments.String("--testrunnerbundleid")
		xctestconfig, _ := arguments.String("--xctestconfig")

		if bundleID == "" && testbundleID == "" && xctestconfig == "" {
			log.Info("no bundle ids specified, falling back to defaults")
			bundleID, testbundleID, xctestconfig = "com.facebook.WebDriverAgentRunner.xctrunner", "com.facebook.WebDriverAgentRunner.xctrunner", "WebDriverAgentRunner.xctest"
		}
		if bundleID == "" || testbundleID == "" || xctestconfig == "" {
			log.WithFields(log.Fields{"bundleid": bundleID, "testbundleid": testbundleID, "xctestconfig": xctestconfig}).Error("please specify either NONE of bundleid, testbundleid and xctestconfig or ALL of them. At least one was empty.")
			return
		}
		log.WithFields(log.Fields{"bundleid": bundleID, "testbundleid": testbundleID, "xctestconfig": xctestconfig}).Info("Running wda")
		go func() {
			err := testmanagerd.RunXCUIWithBundleIds(bundleID, testbundleID, xctestconfig, device)

			if err != nil {
				log.WithFields(log.Fields{"error": err}).Fatal("Failed running WDA")
			}
		}()
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		signal := <-c
		log.Infof("os signal:%d received, closing..", signal)

		err := testmanagerd.CloseXCUITestRunner()
		if err != nil {
			log.Error("Failed closing wda-testrunner")
			os.Exit(1)
		}
		log.Info("Done Closing")
		return
	}

	b, _ = arguments.Bool("ax")
	if b {
		startAx(device)
		return
	}

	b, _ = arguments.Bool("reboot")
	if b {
		err := diagnostics.Reboot(device)
		if err != nil {
			log.Error(err)
		} else {
			log.Info("ok")
		}
		return
	}

}

func fixDevImage(device ios.DeviceEntry, baseDir string) {
	conn, err := imagemounter.New(device)
	exitIfError("failed connecting to image mounter", err)
	signatures, err := conn.ListImages()
	exitIfError("failed getting image list", err)
	if len(signatures) != 0 {
		log.Info("there is already a developer image mounted, reboot the device if you want to remove it. aborting.")
		return
	}
	imagePath, err := imagemounter.DownloadImageFor(device, baseDir)
	exitIfError("failed downloading image", err)
	log.Infof("installing downloaded image '%s'", imagePath)
	mountImage(device, imagePath)

}

func mountImage(device ios.DeviceEntry, path string) {
	conn, err := imagemounter.New(device)
	exitIfError("failed connecting to image mounter", err)
	signatures, err := conn.ListImages()
	exitIfError("failed getting image list", err)
	if len(signatures) != 0 {
		log.Fatal("there is already a developer image mounted, reboot the device if you want to remove it. aborting.")
	}
	err = conn.MountImage(path)
	exitIfError("failed mounting image", err)
	log.WithFields(log.Fields{"image": path, "udid": device.Properties.SerialNumber}).Info("success mounting image")
}

func listMountedImages(device ios.DeviceEntry) {
	conn, err := imagemounter.New(device)
	exitIfError("failed connecting to image mounter", err)
	signatures, err := conn.ListImages()
	exitIfError("failed getting image list", err)
	if len(signatures) == 0 {
		log.Infof("none")
		return
	}
	for _, sig := range signatures {
		log.Infof("%x", sig)
	}
}

func installApp(device ios.DeviceEntry, path string) {
	log.WithFields(
		log.Fields{"appPath": path, "device": device.Properties.SerialNumber}).Info("installing")
	conn, err := zipconduit.New(device)
	exitIfError("failed connecting to zipconduit, dev image installed?", err)
	err = conn.SendFile(path)
	exitIfError("failed writing", err)
}

func language(device ios.DeviceEntry, locale string, language string) {
	lang, err := ios.GetLanguage(device)
	exitIfError("failed getting language", err)

	err = ios.SetLanguage(device, ios.LanguageConfiguration{Language: language, Locale: locale})
	exitIfError("failed setting language", err)
	if lang.Language != language && language != "" {
		log.Debugf("Language should be changed from %s to %s waiting for Springboard to reboot", lang.Language, language)
		notificationproxy.WaitUntilSpringboardStarted(device)
	}
	lang, err = ios.GetLanguage(device)
	exitIfError("failed getting language", err)

	fmt.Println(convertToJSONString(lang))
}

func startAx(device ios.DeviceEntry) {
	go func() {
		deviceList, err := ios.ListDevices()
		exitIfError("failed converting to json", err)

		device := deviceList.DeviceList[0]

		conn, err := accessibility.New(device)
		exitIfError("failed starting ax", err)

		conn.SwitchToDevice()

		conn.EnableSelectionMode()

		for i := 0; i < 3; i++ {
			conn.GetElement()
			time.Sleep(time.Second)
		}
		/*	conn.GetElement()
			time.Sleep(time.Second)
			conn.TurnOff()*/
		//conn.GetElement()
		//conn.GetElement()

		exitIfError("ax failed", err)
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func printVersion() {
	versionMap := map[string]interface{}{
		"version": version,
	}
	if JSONdisabled {
		fmt.Println(version)
	} else {
		fmt.Println(convertToJSONString(versionMap))
	}
}

func startDebugProxy(device ios.DeviceEntry, binaryMode bool) {
	proxy := debugproxy.NewDebugProxy()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Recovered a panic: %v", r)
				proxy.Close()
				debug.PrintStack()
				os.Exit(1)
				return
			}

		}()
		err := proxy.Launch(device, binaryMode)
		log.WithFields(log.Fields{"error": err}).Infof("DebugProxy Terminated abnormally")
		os.Exit(0)
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Info("Shutting down debugproxy")
	proxy.Close()
}

func startForwarding(device ios.DeviceEntry, hostPort int, targetPort int) {
	forward.Forward(device, uint16(hostPort), uint16(targetPort))
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func printDiagnostics(device ios.DeviceEntry) {
	log.Debug("print diagnostics")
	diagnosticsService, err := diagnostics.New(device)
	exitIfError("Starting diagnostics service failed with", err)

	values, err := diagnosticsService.AllValues()
	exitIfError("getting valued failed", err)

	fmt.Println(convertToJSONString(values))
}

func printDeviceDate(device ios.DeviceEntry) {
	allValues, err := ios.GetValues(device)
	exitIfError("failed getting values", err)

	formatedDate := time.Unix(int64(allValues.Value.TimeIntervalSince1970), 0).Format(time.RFC850)
	if JSONdisabled {
		fmt.Println(formatedDate)
	} else {
		fmt.Println(convertToJSONString(map[string]interface{}{"formatedDate": formatedDate, "TimeIntervalSince1970": allValues.Value.TimeIntervalSince1970}))
	}

}
func printInstalledApps(device ios.DeviceEntry, system bool) {
	svc, _ := installationproxy.New(device)
	if !system {
		response, err := svc.BrowseUserApps()
		exitIfError("browsing user apps failed", err)

		log.Info(response)
		return
	}
	response, err := svc.BrowseSystemApps()
	exitIfError("browsing system apps failed", err)

	log.Info(response)
}

func printDeviceName(device ios.DeviceEntry) {
	allValues, err := ios.GetValues(device)
	exitIfError("failed getting values", err)
	if JSONdisabled {
		fmt.Println(allValues.Value.DeviceName)
	} else {
		fmt.Println(convertToJSONString(map[string]string{
			"devicename": allValues.Value.DeviceName,
		}))
	}
}

func saveScreenshot(device ios.DeviceEntry, outputPath string) {
	log.Debug("take screenshot")
	screenshotrService, err := screenshotr.New(device)
	exitIfError("Starting Screenshotr failed with", err)

	imageBytes, err := screenshotrService.TakeScreenshot()
	exitIfError("screenshotr failed", err)

	if outputPath == "" {
		time := time.Now().Format("20060102150405")
		path, _ := filepath.Abs("./screenshot" + time + ".png")
		outputPath = path
	}
	err = ioutil.WriteFile(outputPath, imageBytes, 0777)
	exitIfError("write file failed", err)

	if JSONdisabled {
		fmt.Println(outputPath)
	} else {
		log.WithFields(log.Fields{"outputPath": outputPath}).Info("File saved successfully")
	}
}

func processList(device ios.DeviceEntry) {
	service, err := instruments.NewDeviceInfoService(device)
	defer service.Close()
	if err != nil {
		exitIfError("failed opening deviceInfoService for getting process list", err)
	}
	processList, err := service.ProcessList()
	fmt.Println(convertToJSONString(processList))
}

func printDeviceList(details bool) {
	deviceList, err := ios.ListDevices()
	if err != nil {
		exitIfError("failed getting device list", err)
	}

	if details {
		if JSONdisabled {
			outputDetailedListNoJSON(deviceList)
		} else {
			outputDetailedList(deviceList)
		}
	} else {
		if JSONdisabled {
			fmt.Print(deviceList.String())
		} else {
			fmt.Println(convertToJSONString(deviceList.CreateMapForJSONConverter()))
		}
	}
}

type detailsEntry struct {
	Udid           string
	ProductName    string
	ProductType    string
	ProductVersion string
}

func outputDetailedList(deviceList ios.DeviceList) {
	result := make([]detailsEntry, len(deviceList.DeviceList))
	for i, device := range deviceList.DeviceList {
		udid := device.Properties.SerialNumber
		allValues, err := ios.GetValues(device)
		exitIfError("failed getting values", err)
		result[i] = detailsEntry{udid, allValues.Value.ProductName, allValues.Value.ProductType, allValues.Value.ProductVersion}
	}
	fmt.Println(convertToJSONString(map[string][]detailsEntry{
		"deviceList": result,
	}))
}

func outputDetailedListNoJSON(deviceList ios.DeviceList) {
	for _, device := range deviceList.DeviceList {
		udid := device.Properties.SerialNumber
		allValues, err := ios.GetValues(device)
		exitIfError("failed getting values", err)
		fmt.Printf("%s  %s  %s %s\n", udid, allValues.Value.ProductName, allValues.Value.ProductType, allValues.Value.ProductVersion)
	}
}

func startListening() {
	go func() {
		for {
			deviceConn, err := ios.NewDeviceConnection(ios.DefaultUsbmuxdSocket)
			defer deviceConn.Close()
			if err != nil {
				log.Errorf("could not connect to %s with err %+v, will retry in 3 seconds...", ios.DefaultUsbmuxdSocket, err)
				time.Sleep(time.Second * 3)
				continue
			}
			muxConnection := ios.NewUsbMuxConnection(deviceConn)

			attachedReceiver, err := muxConnection.Listen()
			if err != nil {
				log.Error("Failed issuing Listen command, will retry in 3 seconds", err)
				deviceConn.Close()
				time.Sleep(time.Second * 3)
				continue
			}
			for {
				msg, err := attachedReceiver()
				if err != nil {
					log.Error("Stopped listening because of error")
					break
				}
				fmt.Println(convertToJSONString((msg)))
			}
		}
	}()
	c := make(chan os.Signal, syscall.SIGTERM)
	signal.Notify(c, os.Interrupt)
	<-c
}

func printDeviceInfo(device ios.DeviceEntry, domain, key string) {
	allValues, err := ios.GetDomainValuesPlist(device, domain, key)
	if err != nil {
		exitIfError("failed getting info", err)
	}
	fmt.Println(convertToJSONString(allValues))
}

func runSyslog(device ios.DeviceEntry) {
	log.Debug("Run Syslog.")

	syslogConnection, err := syslog.New(device)
	exitIfError("Syslog connection failed", err)

	defer syslogConnection.Close()

	go func() {
		messageContainer := map[string]string{}
		for {
			logMessage, err := syslogConnection.ReadLogMessage()
			if err != nil {
				exitIfError("failed reading syslog", err)
			}
			logMessage = strings.TrimSuffix(logMessage, "\x00")
			logMessage = strings.TrimSuffix(logMessage, "\x0A")
			if JSONdisabled {
				fmt.Println(logMessage)
			} else {
				messageContainer["msg"] = logMessage
				fmt.Println(convertToJSONString(messageContainer))
			}
		}
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func pairDevice(device ios.DeviceEntry) {
	err := ios.Pair(device)
	if err != nil {
		exitIfError("Pairing failed", err)
	} else {
		log.Infof("Successfully paired %s", device.Properties.SerialNumber)
	}
}

func readPair(device ios.DeviceEntry) {
	record, err := ios.ReadPairRecord(device.Properties.SerialNumber)
	if err != nil {
		exitIfError("failed reading pairrecord", err)
	}
	json, err := json.Marshal(record)
	if err != nil {
		exitIfError("failed converting to json", err)
	}
	fmt.Printf("%s", json)
}

func convertToJSONString(data interface{}) string {
	b, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(b)
}

func exitIfError(msg string, err error) {
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatalf(msg)
	}
}
