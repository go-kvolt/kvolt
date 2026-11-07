package kvolt

import (
	"net"
	"strings"
	"testing"
)

func TestEngine_Run_ListenFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	app := New()
	err = app.Run(ln.Addr().String())
	if err == nil {
		t.Fatal("want listen error when address is already bound")
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("want listen error, got %v", err)
	}
}
