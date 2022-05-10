package ioskit

import (
	"bytes"
	"context"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"net"
	"sync"
)

type RemoteDevice struct {
	Type        wdbd.DeviceType
	Addr        string
	Serial      string
	conn        *grpc.ClientConn
	iosDeviceId int
}

func NewRemoteDevice(ctx context.Context, typ wdbd.DeviceType, addr, serial string) (*RemoteDevice, error) {
	r := &RemoteDevice{
		Type:   typ,
		Addr:   addr,
		Serial: serial,
	}

	err := r.initConn(ctx)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *RemoteDevice) initConn(ctx context.Context) error {
	// Set up a connection to the server.
	conn, err := grpc.DialContext(ctx, r.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err == nil {
		r.conn = conn
	}
	return err
}

func (r *RemoteDevice) GetIOSDeviceId() int {
	return r.iosDeviceId
}

func (r *RemoteDevice) Monitor(ctx context.Context) error {
	client := wdbd.NewWdbdClient(r.conn)
	req := &wdbd.MonitorRequest{
		Device: make([]*wdbd.Device, 1),
	}
	req.Device[0] = &wdbd.Device{
		Uid: r.Serial,
	}

	stream, err := client.StartDeviceMonitor(ctx, req)
	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		log.Infoln("recv: ", event)

		switch event.EventType {
		case wdbd.DeviceEventType_Add:
			r.iosDeviceId = int(event.Device.IosDeviceId)
			globalUsbmuxd.deviceId++
			d := wdbd.DeviceEntry{
				DeviceID:    globalUsbmuxd.deviceId,
				MessageType: ListenMessageAttached,
				Properties: ios.DeviceProperties{
					SerialNumber: event.Device.Uid,
					DeviceID:     globalUsbmuxd.deviceId,
				},
			}
			globalUsbmuxd.registry.AddDevice(ctx, d)
		case wdbd.DeviceEventType_Remove:
			d, err := globalUsbmuxd.registry.DeviceBySerial(event.Device.Uid)
			if err != nil {
				// should not be here
				log.Fatalln("unknown device: ", event)
			}
			globalUsbmuxd.registry.RemoveDevice(ctx, d)
		}
	}
}

type StreamConn struct {
	serial string
	buf    *bytes.Buffer
	stream wdbd.Wdbd_ForwardDeviceClient
}

func (s *StreamConn) Read(p []byte) (n int, err error) {
	if s.buf.Len() == 0 {
		inData, err := s.stream.Recv()
		if err != nil {
			return 0, err
		}
		s.buf.Write(inData.Payload)
	}
	return s.buf.Read(p)
}

func (s *StreamConn) Write(p []byte) (n int, err error) {
	data := &wdbd.DeviceData{
		Device: &wdbd.Device{
			Uid: s.serial,
		},
		Payload: p,
	}

	n = len(p)
	err = s.stream.Send(data)
	return
}

func (r *RemoteDevice) NewStreamConn(ctx context.Context) (io.ReadWriter, error) {
	client := wdbd.NewWdbdClient(r.conn)
	stream, err := client.ForwardDevice(ctx)
	if err != nil {
		return nil, err
	}

	return &StreamConn{
		buf:    bytes.NewBuffer(nil),
		stream: stream,
		serial: r.Serial,
	}, nil
}

func (r *RemoteDevice) Connect(ctx context.Context, sendChan chan []byte, conn net.Conn) error {
	client := wdbd.NewWdbdClient(r.conn)
	stream, err := client.ForwardDevice(ctx)
	if err != nil {
		return err
	}

	data := &wdbd.DeviceData{
		Device: &wdbd.Device{
			Uid: r.Serial,
		},
	}

	wg := sync.WaitGroup{}

	wg.Add(1)
	// write loop
	go func() {
		defer wg.Done()

		for {
			packet, ok := <-sendChan
			if packet == nil || !ok {
				return
			}

			data.Payload = packet
			err := stream.Send(data)
			if err != nil {
				return
			}
		}
	}()

	wg.Add(1)
	// read loop
	go func() {
		defer wg.Done()
		for {
			inData, err := stream.Recv()
			if err != nil {
				return
			}
			_, err = conn.Write(inData.Payload)
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
	return nil
}
