## xpc_parser

解析macOS二进制程序内嵌的plist格式，解析`otool -s <segname> <sectname> <file>`的16进制打印，将其还原为易读的字符串形式。

## 用法

解析headspin内嵌的__info_plist
```
./xpc_parser -file /Library/PrivilegedHelperTools/io.headspin.DevTools.Helper
```

解析headspin内嵌的__launchd_plist

```
./xpc_parser -sectname __launchd_plist -file /Library/PrivilegedHelperTools/io.headspin.DevTools.Helper
```
