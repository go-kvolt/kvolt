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

type nestedAddress struct {
	City string `json:"city"`
}

type profileWithSiblings struct {
	HomeAddress nestedAddress `json:"home_address"`
	WorkAddress nestedAddress `json:"work_address"`
}

type recursiveNode struct {
	Value string         `json:"value"`
	Next  *recursiveNode `json:"next"`
}

func TestSchemaFromSample_SiblingStructReuse(t *testing.T) {
	s := SchemaFromSample(profileWithSiblings{})
	props := s["properties"].(map[string]interface{})
	home := props["home_address"].(map[string]interface{})
	work := props["work_address"].(map[string]interface{})

	if home["type"] != "object" || work["type"] != "object" {
		t.Fatalf("expected objects, got %v and %v", home, work)
	}
	homeProps, okHome := home["properties"].(map[string]interface{})
	workProps, okWork := work["properties"].(map[string]interface{})
	if !okHome || homeProps["city"] == nil {
		t.Fatal("home address properties missing")
	}
	if !okWork || workProps["city"] == nil {
		t.Fatal("work address properties missing due to premature seen map retention")
	}
}

func TestSchemaFromSample_RecursiveStruct(t *testing.T) {
	s := SchemaFromSample(recursiveNode{})
	props := s["properties"].(map[string]interface{})
	if props["value"] == nil {
		t.Fatal("value property missing")
	}
	next := props["next"].(map[string]interface{})
	if next["type"] != "object" {
		t.Fatalf("next type want object, got %v", next["type"])
	}
}
