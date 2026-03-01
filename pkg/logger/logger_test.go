package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogger_New(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	l := New(buf, INFO)
	if l == nil {
		t.Fatal("New: want non-nil Logger")
	}
}

func TestLogger_Default(t *testing.T) {
	l := Default()
	if l == nil {
		t.Fatal("Default: want non-nil Logger")
	}
}

func TestLogger_Info(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	l := New(buf, INFO)
	l.Info("hello")
	out := buf.String()
	if out == "" {
		t.Error("Info: expected output")
	}
	var entry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entry); err != nil {
		t.Errorf("Info: output should be JSON: %v", err)
	}
	if entry.Message != "hello" || entry.Level != "INFO" {
		t.Errorf("Info: entry %+v", entry)
	}
}

func TestLogger_LevelFilter(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	l := New(buf, WARN)
	l.Info("should not appear")
	l.Error("error msg", errDummy{})
	if buf.Len() == 0 {
		t.Error("Error should produce output")
	}
	l2 := New(buf, DEBUG)
	l2.Debug("debug")
}

type errDummy struct{}

func (e errDummy) Error() string { return "dummy" }

func TestLevel_String(t *testing.T) {
	if DEBUG.String() != "DEBUG" || INFO.String() != "INFO" {
		t.Error("Level.String mismatch")
	}
}
