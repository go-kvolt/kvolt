package swagger

import (
	"encoding/json"
	"strings"
	"testing"
)

type createUser struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Age   int    `json:"age"`
}

func TestSchemaFromSample_Struct(t *testing.T) {
	s := SchemaFromSample(createUser{})
	if s["type"] != "object" {
		t.Fatalf("type: %v", s["type"])
	}
	props := s["properties"].(map[string]interface{})
	if props["email"].(map[string]interface{})["type"] != "string" {
		t.Fatal("email")
	}
	req := s["required"].([]string)
	joined := strings.Join(req, ",")
	if !strings.Contains(joined, "email") || !strings.Contains(joined, "age") {
		t.Fatalf("required: %v", req)
	}
	if strings.Contains(joined, "name") {
		t.Fatal("name should not be required (omitempty)")
	}
}

func TestGenerateOpenAPI_WithSchemas(t *testing.T) {
	routes := []RouteInfo{{
		Method:  "POST",
		Path:    "/users",
		Summary: "Create user",
		Tags:    []string{"users"},
		Request: createUser{},
		Responses: map[int]interface{}{
			201: createUser{},
		},
	}}
	raw := generateOpenAPI(routes, "Test")
	var spec map[string]interface{}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	paths := spec["paths"].(map[string]interface{})
	post := paths["/users"].(map[string]interface{})["post"].(map[string]interface{})
	if post["summary"] != "Create user" {
		t.Fatal(post["summary"])
	}
	if _, ok := post["requestBody"]; !ok {
		t.Fatal("missing requestBody")
	}
	tags := post["tags"].([]interface{})
	if tags[0] != "users" {
		t.Fatal(tags)
	}
}
