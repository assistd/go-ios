package services_test

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
)

func newMounterService(t *testing.T) *services.MobileImageMounterService {
	device, err := ios.GetDevice("")
	if err != nil {
		t.Fatal(err)
		return nil
	}

	svc, err := services.NewMobileImageMounterService(device)
	if err != nil {
		t.Fatal(err)
		return nil
	}

	return svc
}
func TestMobileImageMounterServiceQueryDeveloperModeStatus(t *testing.T) {
	svc := newMounterService(t)
	if svc == nil {
		return
	}
	defer svc.Close()

	status, err := svc.QueryDeveloperModeStatus()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("mode: %v", status)
}

func TestMobileImageMounterServiceCopyDevices(t *testing.T) {
	svc := newMounterService(t)
	if svc == nil {
		return
	}
	defer svc.Close()

	_, err := svc.CopyDevices()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMobileImageMounterServiceLookup(t *testing.T) {
	svc := newMounterService(t)
	if svc == nil {
		return
	}
	defer svc.Close()

	_, err := svc.LookupImage()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMobileImageMounterServiceUmount(t *testing.T) {
	svc := newMounterService(t)
	if svc == nil {
		return
	}
	defer svc.Close()

	err := svc.Umount()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMobileImageMounterServiceMount(t *testing.T) {
	svc := newMounterService(t)
	if svc == nil {
		return
	}
	defer svc.Close()

	err := svc.Mount("/Users/wetest/workplace/udt/third_party/go-ios/devimages/16.0/DeveloperDiskImage.dmg",
		"/Users/wetest/workplace/udt/third_party/go-ios/devimages/16.0/DeveloperDiskImage.dmg.signature")
	if err != nil {
		t.Fatal(err)
	}
}
