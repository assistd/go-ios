package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -Wl,-sectcreate,__TEXT,__info_plist,${SRCDIR}/Info.plist
#import <Foundation/Foundation.h>
void hello() {
    NSLog(@"Hello World");
}
*/
import "C"

func main() {
	C.hello()
}
