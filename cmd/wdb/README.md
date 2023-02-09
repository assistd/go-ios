# wdb

提供将远程ios设备投射到本地的功能

# 使用

```
./wdb -addr="remote usbmuxd addr" -udid="remote device addr" -usbmuxd-path="local usbmuxd path" -mode="wdbd" -keepalive=true
```
- addr 表示远程usbmuxd的地址，一般为haproxy等四层转发服务将mac的usbmuxd通过网络ip:port的方式暴露的地址
- udid 表示要连接的远程iOS设备的udid
- usbmuxd-path 表示本地监听的usbmuxd socket，支持tcp和unix模式，eg: -usbmuxd-path=tcp:127.1.1:27015 、unix:/var/run/usbmuxd
- mode 表示运行模式，默认值为wdbd，当前支持模式wdbd(服务端)、wdb(客户端)两种模式
- keepalive 表示是否要将wdbd与wdb之间的连接保持自动保活，注意，两端的keepalive值必须保持一致，默认为false

