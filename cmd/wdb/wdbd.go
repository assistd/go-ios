package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/go-ios/wdbd"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"strings"
	"sync"
)

const messageMaxSize = 512 * 1024

type Wdbd struct {
	// Registry of all the discovered devices.
	registry *wdbd.Registry
	network  string // tcp or uds
	addr     string // port or /var/run/usbmuxd
	stopped  bool
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

func (s *Wdbd) handleMessage(ctx context.Context, msg *wdbd.Request, conn net.Conn) {
	switch msg.Message.(type) {
	case *wdbd.Request_Monitor:
		s.startDeviceMonitor(ctx, msg.GetMonitor(), conn) //blocking
	case *wdbd.Request_List:
		s.listDevices(ctx, msg.GetList(), conn)
	case *wdbd.Request_Forward:
		s.forwardConn(ctx, conn)
	default:
		break
	}
}

func (s *Wdbd) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		log.Warnln("close connection")
	}()

	lenbuf := make([]byte, 4)
	var buf []byte
	for {
		_, err := io.ReadFull(conn, lenbuf)
		if err != nil {
			break
		}
		bodyLen := int(binary.BigEndian.Uint32(lenbuf))
		if bodyLen > messageMaxSize {
			break
		}

		if bodyLen > len(buf) {
			buf = make([]byte, bodyLen)
		}
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			break
		}
		var msg wdbd.Request
		err = proto.Unmarshal(buf, &msg)
		if err != nil {
			break
		}

		s.handleMessage(ctx, &msg, conn)
	}

	return
}

func (s *Wdbd) Run(ctx context.Context, port int) error {
	var listener net.Listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Errorf("fail to listen on: %v, error:%v", port, err)
		return err
	}

	log.Debugf("listen on: %v", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			// FIXME: may need check err type
			log.Errorf("error: listen accept: %v", err)
			break
		}
		go s.handleConn(ctx, conn)
	}

	return nil
}

func (s *Wdbd) startDeviceMonitor(ctx context.Context, req *wdbd.MonitorRequest, stream io.WriteCloser) error {
	log.Infof("wdbd: StartDeviceMonitor enter:%v", req.Device)

	ctx2, cancel := context.WithCancel(ctx)

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

	onAdd := func(ctx2 context.Context, device wdbd.DeviceEntry) {
		if noFilter || filter(device.Properties.SerialNumber) {
			msg := &wdbd.Response{
				Message: &wdbd.Response_Event{
					Event: &wdbd.DeviceEvent{
						EventType: wdbd.DeviceEventType_Add,
						Device: &wdbd.Device{
							DevType:     wdbd.DeviceType_IOS,
							Uid:         device.Properties.SerialNumber,
							IosDeviceId: int32(device.DeviceID),
						},
					},
				},
			}

			b, err := proto.Marshal(msg)
			if err == nil {
				_, err = stream.Write(b)
				if err == nil {
					return
				}
			}

			log.Errorf("wdbd: write msg %+v failed: %v", msg, err)
			cancel()
			stream.Close()
		}
	}

	onRemove := func(ctx2 context.Context, device wdbd.DeviceEntry) {
		if noFilter || filter(device.Properties.SerialNumber) {
			msg := &wdbd.Response{
				Message: &wdbd.Response_Event{
					Event: &wdbd.DeviceEvent{
						EventType: wdbd.DeviceEventType_Remove,
						Device: &wdbd.Device{
							DevType: wdbd.DeviceType_IOS,
							Uid:     device.Properties.SerialNumber,
						},
					},
				},
			}
			log.Infof("send: %+v", msg)
			b, err := proto.Marshal(msg)
			if err == nil {
				_, err = stream.Write(b)
				if err == nil {
					return
				}
			}

			log.Errorf("wdbd: msg %+v failed: %v", msg, err)
			cancel()
			stream.Close()
		}
	}

	for _, device := range s.registry.Devices() {
		onAdd(ctx2, device)
	}

	unregister := s.registry.Listen(wdbd.NewDeviceListener(onAdd, onRemove))
	<-ctx2.Done()
	unregister()
	stream.Close()

	log.Warnln("wdbd: device monitor stream closed")
	return nil
}

func (s *Wdbd) listDevices(ctx context.Context, req *wdbd.ListDevicesRequest, stream io.WriteCloser) error {
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

	msg := &wdbd.Response{
		Message: &wdbd.Response_DeviceList{
			DeviceList: &wdbd.DeviceList{List: deviceList},
		},
	}

	b, err := proto.Marshal(msg)
	if err == nil {
		_, err = stream.Write(b)
		if err == nil {
			return nil
		}
	}
	stream.Close()
	return err
}

func (s *Wdbd) Conn() (net.Conn, error) {
	return net.Dial(s.network, s.addr)
}

func (s *Wdbd) forwardConn(ctx context.Context, stream io.ReadWriteCloser) error {
	log.Warnln("wdbd: ForwardDevice enter")

	devConn, err := net.Dial(s.network, s.addr)
	if err != nil {
		return err
	}
	defer devConn.Close()

	ctx2, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(2)
	closed := false
	go func() {
		io.Copy(devConn, stream)
		if ctx2.Err() == nil {
			cancel()
			devConn.Close()
			stream.Close()
			closed = true
		}
		wg.Done()
	}()
	go func() {
		io.Copy(stream, devConn)
		if ctx2.Err() == nil {
			cancel()
			devConn.Close()
			stream.Close()
			closed = true
		}
		wg.Done()
	}()

	<-ctx.Done()
	if !closed {
		devConn.Close()
		stream.Close()
	}
	wg.Wait()

	log.Warnln("wdbd: device forward stream closed")
	return nil
}
