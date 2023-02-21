package services

import "github.com/danielpaulus/go-ios/ios"

type RemoteServer struct {
	BaseService
}

func NewRemoteServer(device ios.DeviceEntry, name string) (*RemoteServer, error) {
	s := &RemoteServer{
		BaseService: BaseService{
			Name:        name,
			IsDeveloper: true,
		},
	}
	err := s.init(device)
	return s, err
}

func (r *RemoteServer) Init() {

}
