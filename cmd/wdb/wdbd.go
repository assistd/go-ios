package main

import (
	"context"
	"fmt"
	"github.com/danielpaulus/go-ios/wdbd"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
	"net"
	"strings"
)

type Wdbd struct {
	// Registry of all the discovered devices.
	registry *wdbd.Registry
	network  string // tcp or uds
	addr     string // port or /var/run/usbmuxd
	wdbd.UnimplementedWdbdServer
}

func NewWdbd(socket string) (*Wdbd, error) {
	pos := strings.Index(socket, ":")
	if pos < 0 {
		return nil, fmt.Errorf("invalid socket: %v", socket)
	}

	network, addr := socket[0:pos], socket[pos+1:]
	return &Wdbd{
		network:  network,
		addr:     addr,
		registry: wdbd.NewRegistry(),
	}, nil
}

func (s *Wdbd) Monitor(ctx context.Context) {
	kit, err := ioskit.NewDeviceMonitor(s.network, s.addr)
	if err != nil {
		log.Fatalln("wdbd create failed: ", err)
	}

	err = kit.Monitor(ctx, s.registry, -1)
	if err != nil {
		log.Fatalln("wdbd quit")
	}
}

func (s *Wdbd) StartDeviceMonitor(req *wdbd.MonitorRequest, stream wdbd.Wdbd_StartDeviceMonitorServer) error {
	log.Infof("wdbd: StartDeviceMonitor enter:%v", req.Device)

	devices := req.GetDevice()
	noFilter := false
	if devices == nil {
		noFilter = true
	}
	filter := func(serial string) bool {
		for _, d := range devices {
			if d.Uid == serial {
				return true
			}
		}

		return false
	}

	onAdd := func(ctx context.Context, device wdbd.DeviceEntry) {
		msg := &wdbd.DeviceEvent{
			EventType: wdbd.DeviceEventType_Add,
			Device: &wdbd.Device{
				DevType:     wdbd.DeviceType_IOS,
				Uid:         device.Properties.SerialNumber,
				IosDeviceId: int32(device.DeviceID),
			},
		}

		if noFilter || filter(device.Properties.SerialNumber) {
			log.Infof("send: %+v", msg)
			stream.Send(msg)
		}
	}

	onRemove := func(ctx context.Context, device wdbd.DeviceEntry) {
		msg := &wdbd.DeviceEvent{
			EventType: wdbd.DeviceEventType_Remove,
			Device: &wdbd.Device{
				DevType: wdbd.DeviceType_IOS,
				Uid:     device.Properties.SerialNumber,
			},
		}

		if noFilter || filter(device.Properties.SerialNumber) {
			log.Infof("send: %+v", msg)
			stream.Send(msg)
		}
	}

	for _, device := range s.registry.Devices() {
		onAdd(stream.Context(), device)
	}

	s.registry.Listen(wdbd.NewDeviceListener(onAdd, onRemove))

	<-stream.Context().Done()
	log.Warnln("wdbd: device monitor stream closed")
	return nil
}

func (s *Wdbd) ListDevices(ctx context.Context, req *wdbd.ListDevicesRequest) (*wdbd.DeviceList, error) {
	devices := s.registry.Devices()
	deviceList := make([]*wdbd.Device, len(devices))
	for i, d := range devices {
		deviceList[i] = &wdbd.Device{
			DevType:     wdbd.DeviceType_IOS,
			ConnType:    wdbd.DeviceConnType_Usb,
			Uid:         d.Properties.SerialNumber,
			IosDeviceId: int32(d.DeviceID),
		}
	}
	return &wdbd.DeviceList{List: deviceList}, nil
}

func (s *Wdbd) Conn() (net.Conn, error) {
	return net.Dial(s.network, s.addr)
}

func (s *Wdbd) ForwardDevice(stream wdbd.Wdbd_ForwardDeviceServer) error {
	log.Warnln("wdbd: ForwardDevice enter")

	devConn, err := net.Dial(s.network, s.addr)
	if err != nil {
		return err
	}
	defer devConn.Close()

	ctx := stream.Context()
	go func() {
		for {
			data, err := stream.Recv()
			if err != nil {
				log.Errorln("wdbd: forward: receive err: ", err)
				break
			}
			/*
				_, err = registry.DeviceBySerial(data.DeviceEntry.Uid)
				if err != nil {
					log.Errorln("wdbd: forward: device not found: ", data.DeviceEntry.Uid)
					break
				}
				log.Infoln("wdbd: forward to device: ", data.DeviceEntry.Uid)
			*/
			//log.Infoln(hex.Dump(data.Payload))
			if _, err := devConn.Write(data.Payload); err != nil {
				break
			}
		}
	}()

	go func() {
		buf := make([]byte, 512*1024)
		for {
			n, err := devConn.Read(buf)
			if err != nil {
				break
			}

			//log.Infoln(hex.Dump(buf[:n]))
			data := &wdbd.DeviceData{
				Payload: buf[:n],
			}
			err = stream.Send(data)
			if err != nil {
				log.Errorln("wdbd: forward: receive err: ", err)
				break
			}
		}
	}()

	<-ctx.Done()
	log.Warnln("wdbd: device forward stream closed")
	return nil
}
