package security

import (
	"testing"
)

func TestSayHello(t *testing.T) {
	actual := MakeUUID()
	expected := "Hello, world."
	if expected == actual {
		t.Errorf("Error occured while testing sayhello: '%s' != '%s'", expected, actual)
	}
}
