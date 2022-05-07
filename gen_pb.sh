#!/bin/bash

# ref https://developers.google.com/protocol-buffers/docs/gotutorial
# ref https://grpc.io/docs/languages/go/quickstart/
# go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

#list=(
#./core/bind/device.proto
#)
#
#for i in "${list[@]}"; do
#    protoc --go_out=./ --go_opt=paths=source_relative $i
#done

# grpc version 1.1
function grpc_gen {
    #export PATH=`pwd`/toolchain/V1.1:$PATH
    protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative $@
}

grpc_gen wdbd/wdbd.proto
