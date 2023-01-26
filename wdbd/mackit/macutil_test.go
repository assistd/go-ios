package mackit_test

import (
	"testing"

	"github.com/danielpaulus/go-ios/wdbd/mackit"
)

func TestGetUdid(t *testing.T) {
	out, _ := mackit.GetUdid()
	t.Log(out)
}
