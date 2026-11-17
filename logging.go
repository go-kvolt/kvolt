package kvolt

import "github.com/go-kvolt/kvolt/v2/internal/logx"

// SetLogFunc sends KVolt's own status messages to fn instead of standard output.
//
// These are the lines the framework prints about itself: server started and stopped
// (Run, RunTLS, grpc.Run, grpc.RunDual), queue workers started and stopped, the gRPC
// logging / recovery interceptors, and the API docs notice. fn receives one message
// per call, without a trailing newline. It may be called from several goroutines.
//
//	kvolt.SetLogFunc(func(msg string) { slog.Info(msg, "component", "kvolt") })
//
// Pass nil to restore standard output. The setting is process-wide; set it before
// starting servers. Request access logs are configured separately through
// middleware.LoggerConfig.Output, and errors logged with the standard library "log"
// package through log.SetOutput.
func SetLogFunc(fn func(msg string)) {
	logx.Set(fn)
}
