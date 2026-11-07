package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLevel_String_AllAndUnknown(t *testing.T) {
	cases := []struct {
		l    Level
		want string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(99), "UNKNOWN"},
	}
	for _, tc := range cases {
		if got := tc.l.String(); got != tc.want {
			t.Fatalf("%v: got %q want %q", tc.l, got, tc.want)
		}
	}
}

func TestLogger_InfoDebugWithFields(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	l := New(buf, DEBUG)
	l.Info("i", map[string]interface{}{"k": "v"})
	l.Debug("d", map[string]interface{}{"n": 1})
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("want >=2 lines, got %q", out)
	}
	var e LogEntry
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatal(err)
	}
	if e.Fields["k"] != "v" {
		t.Fatalf("fields=%v", e.Fields)
	}
}

func TestLogger_WarnViaLevel(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	l := New(buf, WARN)
	l.log(WARN, "warned", nil)
	if !strings.Contains(buf.String(), "WARN") {
		t.Fatalf("got %q", buf.String())
	}
}
