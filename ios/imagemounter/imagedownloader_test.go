package imagemounter_test

import (
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVersionMatching(t *testing.T) {
	assert.Equal(t, "11.2", imagemounter.MatchAvailable("11.2.5"))
	assert.Equal(t, "12.2 (16E226)", imagemounter.MatchAvailable("12.2.5"))
	assert.Equal(t, "13.6", imagemounter.MatchAvailable("13.6.1"))
	assert.Equal(t, "14.7", imagemounter.MatchAvailable("14.7.1"))
	assert.Equal(t, "15.2", imagemounter.MatchAvailable("15.3.1"))
	assert.Equal(t, "15.4", imagemounter.MatchAvailable("15.4.1"))
	assert.Equal(t, "15.4", imagemounter.MatchAvailable("19.4.1"))
}

func TestIsImageMount(t *testing.T) {
	device, err := ios.GetDevice("b90bf1dc928bca9f3e689bc0ec931ceba781d4d7")
	if err != nil {

	}
	b, err := imagemounter.IsImageMount(device)
	if b {
		log.Infof("device: %s have mounted", device.Properties.SerialNumber)
		return
	}
	log.Errorf("device: %s is not mounted %v", device.Properties.SerialNumber, err)
}
