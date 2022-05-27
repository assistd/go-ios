package ioskit

import (
	"context"
	"encoding/binary"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
)

type RemoteDevice struct {
	Type        wdbd.DeviceType
	Addr        string
	Serial      string
	iosDeviceId int
}

func NewRemoteDevice(ctx context.Context, typ wdbd.DeviceType, addr, serial string) *RemoteDevice {
	return &RemoteDevice{
		Type:   typ,
		Addr:   addr,
		Serial: serial,
	}
}

func (r *RemoteDevice) GetIOSDeviceId() int {
	return r.iosDeviceId
}

func (r *RemoteDevice) Monitor(ctx context.Context) error {
	monitor := &wdbd.MonitorRequest{
		Device: make([]*wdbd.Device, 1),
	}
	monitor.Device[0] = &wdbd.Device{
		Uid: r.Serial,
	}
	req := &wdbd.Request{
		Message: &wdbd.Request_Monitor{
			Monitor: monitor,
		},
	}

	conn, err := net.Dial("tcp", r.Addr)
	if err != nil {
		return err
	}
	defer func() {
		conn.Close()
	}()

	lenbuf := make([]byte, 4)
	b, _ := proto.Marshal(req)
	binary.BigEndian.PutUint32(lenbuf, uint32(len(b)))
	_, err = conn.Write(lenbuf)
	if err != nil {
		return err
	}
	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	var buf []byte
	for {
		_, err := io.ReadFull(conn, lenbuf)
		if err != nil {
			return err
		}
		bodyLen := int(binary.BigEndian.Uint32(lenbuf))
		if bodyLen > len(buf) {
			buf = make([]byte, bodyLen)
		}

		buf2 := buf[:bodyLen]
		log.Infof("wdb: recv: %v", bodyLen)
		_, err = io.ReadFull(conn, buf2)
		if err != nil {
			log.Errorln("wdb: recv failed: ", err)
			break
		}
		var msg wdbd.Response
		err = proto.Unmarshal(buf2, &msg)
		if err != nil {
			log.Errorln("wdb: unmarshal failed: ", err)
			break
		}
		m, ok := msg.Message.(*wdbd.Response_Event)
		if !ok {
			log.Fatalln("not event: ", msg.Message)
		}
		log.Infof("wdb: recv: %v", m)

		switch m.Event.EventType {
		case wdbd.DeviceEventType_Add:
			r.iosDeviceId = int(m.Event.Device.IosDeviceId)
			globalUsbmuxd.deviceId++
			d := wdbd.DeviceEntry{
				DeviceID:    globalUsbmuxd.deviceId,
				MessageType: ListenMessageAttached,
				Properties: ios.DeviceProperties{
					SerialNumber: m.Event.Device.Uid,
					DeviceID:     globalUsbmuxd.deviceId,
				},
			}
			globalUsbmuxd.registry.AddDevice(ctx, d)
		case wdbd.DeviceEventType_Remove:
			d, err := globalUsbmuxd.registry.DeviceBySerial(m.Event.Device.Uid)
			if err != nil {
				// should not be here
				log.Fatalln("unknown device: ", m.Event)
			}
			globalUsbmuxd.registry.RemoveDevice(ctx, d)
		}
	}

	return io.EOF
}

func (r *RemoteDevice) NewConn(ctx context.Context) (net.Conn, error) {
	conn, err := net.Dial("tcp", r.Addr)
	if err != nil {
		return nil, err
	}
	req := &wdbd.Request{
		Message: &wdbd.Request_Forward{
			Forward: &wdbd.ForwardDeviceRequest{},
		},
	}

	lenbuf := make([]byte, 4)
	b, _ := proto.Marshal(req)
	binary.BigEndian.PutUint32(lenbuf, uint32(len(b)))
	_, err = conn.Write(lenbuf)
	if err != nil {
		conn.Close()
		return nil, err
	}
	_, err = conn.Write(b)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
