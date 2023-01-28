#!/bin/bash

# https://github.com/golang/go/issues/28832
# https://clang.llvm.org/docs/genindex.html
# https://discourse.cmake.org/t/how-to-embed-info-plist-in-a-simple-mac-binary-file/512

clang server.c -sectcreate __TEXT __info_plist Info.plist -o com.apple.myservice
otool -s __TEXT __info_plist com.apple.myservice
codesign --deep --force --options=runtime --sign "Developer ID Application: Shan Yu (4TDFARXPF6)" --timestamp com.apple.myservice
sudo cp com.apple.myservice /Library/PrivilegedHelperTools/
