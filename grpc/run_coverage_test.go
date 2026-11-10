package grpc

import (
	"net"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2"
)

func TestRun_HappyPathStop(t *testing.T) {
	e := kvolt.New()
	s := Attach(e)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- Run(e, addr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	s.GracefulStop()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Run")
	}
}

func TestRun_QuitHook(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	quit := make(chan struct{})
	testRunQuit = quit
	defer func() { testRunQuit = nil }()

	done := make(chan error, 1)
	go func() { done <- Run(e, addr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(quit)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestRunDual_QuitHook(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpAddr := httpLn.Addr().String()
	_ = httpLn.Close()
	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcAddr := grpcLn.Addr().String()
	_ = grpcLn.Close()

	quit := make(chan struct{})
	testRunQuit = quit
	defer func() { testRunQuit = nil }()

	done := make(chan error, 1)
	go func() { done <- RunDual(e, httpAddr, grpcAddr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", httpAddr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(quit)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}
