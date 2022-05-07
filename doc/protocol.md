# 参考资料

- https://zyqhi.github.io/2019/08/20/usbmuxd-protocol.html
- https://jon-gabilondo-angulo-7635.medium.com/understanding-usbmux-and-the-ios-lockdown-service-7f2a1dfd07ae
- https://km.woa.com/group/24938/articles/show/468187?kmref=search&from_page=1&no=2
- https://blog.csdn.net/leonpengweicn/article/details/8585857
- https://www.theiphonewiki.com/wiki/Usbmux

# 协议细节

```
struct header{}; 
char payload[]; // 负载
```

## header

与usbmuxd通信，来回数据都被添加协议头，共16个字节

```go
type UsbMuxHeader struct {
	Length  uint32
	Version uint32
	Request uint32
	Tag     uint32
}
```

- length，消息长度，包括16字节的头部
- version，协议版本，目前使用的是1
- type/request，协议类型，目前固定是8
- tag，消息编号，针对该请求的响应消息会使用此tag


相关代码参见：usbmuxconnection.go

## payload

payload使用plist（一种xml格式变体）来封装。

## ListDevices

host->device

```json
{
	"ClientVersionString": "libusbmuxd 1.1.0",
	"MessageType": "ListDevices",
	"ProgName": "tidevice",
	"kLibUSBMuxVersion": 3
}
```

device->host

```json
{
	"DeviceList": [
		{
			"DeviceID": 1,
			"MessageType": "Attached",
			"Properties": {
				"ConnectionSpeed": 480000000,
				"ConnectionType": "USB",
				"DeviceID": 1,
				"LocationID": 17891328,
				"ProductID": 4776,
				"SerialNumber": "f1affa7e8e5e47e7e4bc1180543d7f599eff2716",
				"USBSerialNumber": "f1affa7e8e5e47e7e4bc1180543d7f599eff2716"
			}
		}
	]
}

```

至此跟usbmuxd的链路结束。

# lockdown协议

## Connect

host->device

```json
{
	"DeviceID": 1,
	"MessageType": "Connect",
	"PortNumber": 32498,
	"ProgName": "tidevice"
}
```

host->device

```json
{
	"Key": "DeviceName",
	"Label": "tidevice",
	"Request": "GetValue"
}
```

device->host

```json
{
	"Key": "DeviceName",
	"Request": "GetValue",
	"Value": "iPhone"
}
```

## SystemBUID

```xml
<key>SystemBUID</key><string>B0911AB5–84F7–436F-936E-DEA460F6EA3A</string>
```

This refers to the ID of the computer that runs iTunes.

疑问：BUID是如何生成的，以及BUID能同时信任两台或者多台电脑吗？

## 配对问题

PC client -> usbmuxd

```
REQ: ReadPairRecord PairRecorID “a recordID”
RESP: PairRecordData with the Data (PairRecord={DeviceCertificate=xxxx,HostCertificate=xxxx,HostID=xxxx,RootCertificate=xxxx})
```

这里返回的PC的公钥还是手机的公钥？
不确定

usbmuxd会读取`/var/lib/lockdown/<udid>.plist`文件 `/var/db/lockdown/<udid>.plist`文件，并返回给client

pair的时候，先向usbmuxd请求BUID，这个BUID是usbmuxd首次启动的时候生成在`SystemConfiguration.plist`中的。

**电脑**

<udid>.plist，其中有个字段

```xml
	<key>HostID</key>
	<string>55B63BC5-CE5C-4CC9-BFED-6278F9B4E5AC</string>

	<key>SystemBUID</key>
	<string>04F70480-FD26-49D2-8FAA-712847ACAAF0</string>
```

这个SystemBUID跟`SystemConfiguration.plist`中的SystemBUID一致。代表的是这台macmini。
那么这个HostID，代表是手机还是电脑？我认为它代表的是这台mac。

**手机上**

/var/root/Library/Lockdown/pair_records，存放二进制格式的plist，可以使用vscode的plist插件来查看。
存放`<HostID>.plist`，每个文件对应一台mac，可以有多个。

1. client向usbmuxd获取SystemBUID
2. client向手机的lockdown申请DevicePublicKey和WiFiAddress
3. client利用lockdown返回的DevicePublicKey，构造出rootCert, hostCert, deviceCert, rootPrivateKey, hostPrivateKey
4. client，创建一个新的udid作为hostid，并利用buid, hostCert, rootCert, deviceCert构造pairRecordData，向lockdown请求
5. lockdown返回PairingDialogResponsePending错误
6. 在手机点击“信任此电脑“，此时lockdown不会保存<hostid>.plist
7. client重新走pair流程，并不会报错误，利用`SavePairRecord`指挥usbmuxd保存此，手机上lockdown利用第3步client创建的HostID保存<HostID>.plist

神奇之处在于，这个HOST-ID是由./go-ios pair命令，再第二次执行时生成的。

macOS上也要点击信任

## forward接口测试



## 阅读usbmuxd源码

关键函数

- config_get_config_dir
- config_set_device_record
- config_get_device_record