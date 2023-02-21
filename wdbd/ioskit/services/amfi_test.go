package services_test

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
)

func NewAmfiService(t *testing.T) *services.AmfiService {
	device, err := ios.GetDevice("")
	if err != nil {
		t.Fatal(err)
		return nil
	}

	svc, err := services.NewAmfiService(device)
	if err != nil {
		t.Fatal(err)
		return nil
	}

	return svc
}
func TestAmfiServiceEnableDeveloperMode(t *testing.T) {
	svc := NewAmfiService(t)
	if svc == nil {
		return
	}
	defer svc.Close()

	err := svc.EnableDeveloperMode(true)
	if err != nil {
		t.Fatal(err)
	}
}
