package swagger

import (
	"strings"
	"testing"
)

type CommonHeader struct {
	TraceID string `json:"trace_id"`
	Version int    `json:"version"`
}

type PayloadWithEmbedded struct {
	CommonHeader
	Action string `json:"action"`
}

func TestSchemaFromSample_EmbeddedStructFlattened(t *testing.T) {
	s := SchemaFromSample(PayloadWithEmbedded{})
	if s["type"] != "object" {
		t.Fatalf("expected type=object, got %v", s["type"])
	}

	props, ok := s["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %v", s["properties"])
	}

	// Embedded fields should appear directly on top-level properties
	if props["trace_id"] == nil {
		t.Fatal("expected trace_id on top-level properties")
	}
	if props["version"] == nil {
		t.Fatal("expected version on top-level properties")
	}
	if props["action"] == nil {
		t.Fatal("expected action on top-level properties")
	}

	// Should not have "CommonHeader" as a nested property
	if props["CommonHeader"] != nil {
		t.Fatal("CommonHeader should be flattened, not present as a nested property")
	}

	req, ok := s["required"].([]string)
	if !ok {
		t.Fatalf("expected required slice, got %v", s["required"])
	}
	reqJoined := strings.Join(req, ",")
	if !strings.Contains(reqJoined, "trace_id") || !strings.Contains(reqJoined, "action") {
		t.Fatalf("expected trace_id and action in required, got %v", req)
	}
}
