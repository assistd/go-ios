## xpc

- https://github.com/jceel/libxpc
- http://davidleee.com/2020/07/20/ipc-for-macOS/
- https://juejin.cn/post/6844904170386882568

```
$ ./wdb com.apple.myservice
[general] Received message in generic event handler: 0x148606d10
[general] <dictionary: 0x148606d10> { count = 1, transaction: 1, voucher = 0x148606ba0, contents =
        "foo" => <string: 0x148606e10> { length = 3, contents = "bar" }
}
[general] Received message in generic event handler: 0x148606cd0
[general] <dictionary: 0x148606cd0> { count = 1, transaction: 1, voucher = 0x148606ba0, contents =
        "foo" => <string: 0x148606dc0> { length = 3, contents = "bar" }
}
[general] Received message in generic event handler: 0x148606cd0
[general] <dictionary: 0x148606cd0> { count = 1, transaction: 1, voucher = 0x148606ba0, contents =
        "foo" => <string: 0x148606dd0> { length = 3, contents = "bar" }
}
[2]: Received second message: 0x20b91a4b0
[2]: <dictionary: 0x20b91a4b0> { count = 1, transaction: 0, voucher = 0x0, contents =
        "XPCErrorDescription" => <string: 0x20b91a6a0> { length = 22, contents = "Connection interrupted" }
}
[3] Received third message: 0x20b91a4b0
[3] <dictionary: 0x20b91a4b0> { count = 1, transaction: 0, voucher = 0x0, contents =
        "XPCErrorDescription" => <string: 0x20b91a6a0> { length = 22, contents = "Connection interrupted" }
```

原因已经清楚，因为server没有针对xpc_connection_send_message_with_reply创建relay。
具体可以参考`man xpc_connection_send_message`

```

     CLIENT SIDE

           xpc_connection_send_message_with_reply(connection, message, replyq, ^(xpc_object_t reply) {
                   if (xpc_get_type(reply) == XPC_TYPE_DICTIONARY) {
                           // Process reply message that is specific to the message sent.
                   } else {
                           // There was an error, indicating that the caller will never receive
                           // a reply to this message. Tear down any associated data structures.
                   }
           });

     SERVICE SIDE

           void
           handle_message(xpc_object_t message)
           {
                   if (xpc_dictionary_get_bool(message, "ExpectsReply")) {
                           // Sender has set the protocol-defined "ExpectsReply" key, and therefore
                           // it expects the reply to be delivered specially.
                           xpc_object_t reply = xpc_dictionary_create_reply(message);
                           // Populate 'reply' as a normal dictionary.

                           // This is the connection from which the message originated.
                           xpc_connection_t remote = xpc_dictionary_get_remote_connection(message);
                           xpc_connection_send_message(remote, reply);
                           xpc_release(reply);
                   } else {
                           // The sender does not expect any kind of special reply.
                   }
           }
```

## 签名

微盘：WeTest终端实验室-开发资料-iOS开发者证书-yushan

【腾讯文档】macOS app签名
https://docs.qq.com/doc/DVXlZbUpCa0paTVVw

## cgo

- https://github.com/golang/go/issues/28832
- https://coderwall.com/p/l9jr5a/accessing-cocoa-objective-c-from-go-with-cgo
- https://medium.com/using-go-in-mobile-apps/using-go-in-mobile-apps-part-4-calling-objective-c-from-go-8ec801d5d04e

- https://zhuanlan.zhihu.com/p/349197066
- https://chai2010.cn/advanced-go-programming-book/ch2-cgo/ch2-02-basic.html

```
$ cd cmd/xpc
$ go build -x main.go
```

## lipo

lipo命令：https://ss64.com/osx/lipo.html

lipo <fat-binary> -thin x86_64 -output xxx.x64
