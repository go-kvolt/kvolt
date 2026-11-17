// Package logx routes KVolt's own status messages (server started, shutting down,
// queue started, ...) to one place. By default they are printed to standard output;
// kvolt.SetLogFunc replaces the destination for the whole process.
package logx

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

var sink atomic.Pointer[func(msg string)]

// Set replaces the destination. nil restores the default (standard output).
func Set(fn func(msg string)) {
	if fn == nil {
		sink.Store(nil)
		return
	}
	sink.Store(&fn)
}

// Printf formats a status message and sends it to the destination.
func Printf(format string, args ...any) {
	emit(fmt.Sprintf(format, args...))
}

// Println is the fmt.Println counterpart of Printf.
func Println(args ...any) {
	emit(fmt.Sprintln(args...))
}

func emit(msg string) {
	if fn := sink.Load(); fn != nil {
		// A custom destination gets the bare message: the blank lines and trailing
		// newline exist only to lay the text out on a terminal.
		if msg = strings.TrimSpace(msg); msg != "" {
			(*fn)(msg)
		}
		return
	}
	_, _ = fmt.Fprint(os.Stdout, msg)
}
