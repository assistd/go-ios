package main

const (
	MuxMessageTypeListen           = "Listen"
	MuxMessageTypeConnect          = "Connect"
	MuxMessageTypeListDevices      = "ListDevices"
	MuxMessageTypeListListeners    = "ListListeners"
	MuxMessageTypeReadBUID         = "ReadBUID"
	MuxMessageTypeReadPairRecord   = "ReadPairRecord"
	MuxMessageTypeSavePairRecord   = "SavePairRecord"
	MuxMessageTypeDeletePairRecord = "DeletePairRecord"
)

const (
	ResultOK          = 0
	ResultBadCommand  = 1
	ResultBadDev      = 2
	ResultConnRefused = 3
	ResultBadVersion  = 6
)

const (
	MessageResult       = 1
	MessageConnect      = 2
	MessageListen       = 3
	MessageDeviceAdd    = 4
	MessageDeviceRemove = 5
	MessageDevicePaired = 6
	MessagePlist        = 8
)

const (
	ListenMessageAttached = "Attached"
	ListenMessageDetached = "Detached"
	ListenMessagePaired   = "Paired"
)
