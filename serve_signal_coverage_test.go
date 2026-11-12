package kvolt

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeAndWait_EmptyAddrDefault(t *testing.T) {
	app := New()
	quit := make(chan struct{})
	testServeQuit = quit
	defer func() { testServeQuit = nil }()

	srv := &http.Server{} // empty Addr → serveAndWait sets ":http"
	done := make(chan error, 1)
	go func() { done <- app.RunWithServer(srv) }()

	select {
	case err := <-done:
		if srv.Addr != ":http" {
			t.Fatalf("Addr after defaulting=%q", srv.Addr)
		}
		if err == nil {
			t.Fatal("want listen error for :http (or unexpected success)")
		}
	case <-time.After(400 * time.Millisecond):
		close(quit)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout shutting down")
		}
		if srv.Addr != ":http" {
			t.Fatalf("Addr after defaulting=%q", srv.Addr)
		}
	}
}

func TestServeAndWait_GracefulQuitHook(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	quit := make(chan struct{})
	testServeQuit = quit
	defer func() { testServeQuit = nil }()

	app := New()
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
	close(quit)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for graceful quit")
	}
}
