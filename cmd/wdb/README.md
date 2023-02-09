# wdb

提供将远程ios设备投射到本地的功能

## 使用

```
./wdb -addr="remote usbmuxd addr" -udid="udid" -usbmuxd-path="local usbmuxd path" -mode="wdbd" -keepalive
```

- addr 表示远程usbmuxd协议地址，一般为haproxy等四层转发服务将mac的usbmuxd通过网络ip:port的方式暴露的地址
- udid 表示要连接的远程iOS设备的udid
- usbmuxd-path 设置usbmuxd监听的socket地址，支持`tcp`和`unix domain socket`两种，典型用法如下
    - windows: `-usbmuxd-path=tcp:0.0.0.0:27015`
    - mac/linux: `-usbmuxd-path=unix:/var/run/usbmuxd`
- mode 设置运行模式，支持以下两种模式
    - wdbd，默认模式，此时工作于usbmuxd协议解析模式
    - wdb，socat转发模式
- keepalive 表示是否要将wdbd与wdb之间的连接保持自动保活，注意两端保活状态需保持一致，保活间隔1小时，默认为false

## 典型用法

1. 局域网使用

```
    ┌──────────────────┐           ┌─────────────────┐
    │    user's PC     │           │  macOS/windows  │   ┌───┐
    │                  │  network  │                 │   │ios│
    │  wdb -mode wdbd  │◄─────────►│ haproxy usbmuxd ├───┤   │
    │                  │           │ realm           │   └───┘
    └──────────────────┘           └─────────────────┘
```

- 电脑上安装Haproxy（macOS或Linux）或者realm（windows上使用），将usmbuxd监听的服务暴露出来。
- `./wdb -addr="remote usbmuxd addr" -udid="udid"`

2. 内网-公网服务器-内网架构

```
    ┌──────────────────┐            ┌──────────────────┐           ┌─────────────────┐
    │    user's PC     │            │      server      │           │  macOS/windows  │   ┌───┐
    │                  │  network   │                  │  network  │                 │   │ios│
    │  wdb -mode wdb   │◄─────────► │  wdb -mode wdbd  │◄─────────►│ haproxy usbmuxd ├───┤   │
    │                  │            │                  │           │ realm           │   └───┘
    └──────────────────┘            └──────────────────┘           └─────────────────┘
```

- server: `./wdb -addr="remote-mac/win-ip-port" -udid="udid" -usbmuxd-path="tcp:0.0.0.0:<port>" -keepalive`
- user's PC: `./wdb -addr="server-ip:port" -mode wdb -keepalive`