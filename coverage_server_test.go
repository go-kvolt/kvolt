package kvolt

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunWithServerNilAndShutdown(t *testing.T) {
	app := New()
	if err := app.RunWithServer(nil); err == nil {
		t.Fatal("want nil server error")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := &http.Server{Addr: addr}
	done := make(chan error, 1)
	go func() { done <- app.RunWithServer(srv) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	shCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for RunWithServer")
	}
}

func TestListenAndServeBindError(t *testing.T) {
	app := New()
	if err := app.ListenAndServe("not a host:abc"); err == nil {
		t.Fatal("want listen error")
	}
}

func TestRunTLSBadCert(t *testing.T) {
	app := New()
	errCh := make(chan error, 1)
	go func() { errCh <- app.RunTLS("127.0.0.1:0", "missing.crt", "missing.key") }()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("want TLS error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestListenAndServeAndShutdown(t *testing.T) {
	app := New()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	gotCh := make(chan net.Listener, 1)
	testListenHook = func(l net.Listener) { gotCh <- l }
	defer func() { testListenHook = nil }()

	done := make(chan error, 1)
	go func() { done <- app.ListenAndServe(addr) }()

	var got net.Listener
	select {
	case got = <-gotCh:
	case <-time.After(2 * time.Second):
		t.Fatal("hook not called")
	}
	_ = got.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("serve hang")
	}
}
