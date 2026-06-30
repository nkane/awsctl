package dynamo

import (
	"testing"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestJSONToItem(t *testing.T) {
	src := `{
		"pk": "user#1",
		"sk": "profile",
		"age": 42,
		"score": 3.1400,
		"active": true,
		"deleted": null,
		"tags": ["a", "b"],
		"meta": { "city": "NYC", "zip": 10001 }
	}`
	item, err := jsonToItem(src)
	if err != nil {
		t.Fatalf("jsonToItem: %v", err)
	}

	if s, ok := item["pk"].(*ddbtypes.AttributeValueMemberS); !ok || s.Value != "user#1" {
		t.Errorf("pk: want S=user#1, got %#v", item["pk"])
	}
	// Numbers must be preserved exactly (no float reformatting).
	if n, ok := item["age"].(*ddbtypes.AttributeValueMemberN); !ok || n.Value != "42" {
		t.Errorf("age: want N=42, got %#v", item["age"])
	}
	if n, ok := item["score"].(*ddbtypes.AttributeValueMemberN); !ok || n.Value != "3.1400" {
		t.Errorf("score: want N=3.1400 (exact), got %#v", item["score"])
	}
	if b, ok := item["active"].(*ddbtypes.AttributeValueMemberBOOL); !ok || !b.Value {
		t.Errorf("active: want BOOL=true, got %#v", item["active"])
	}
	if _, ok := item["deleted"].(*ddbtypes.AttributeValueMemberNULL); !ok {
		t.Errorf("deleted: want NULL, got %#v", item["deleted"])
	}
	l, ok := item["tags"].(*ddbtypes.AttributeValueMemberL)
	if !ok || len(l.Value) != 2 {
		t.Fatalf("tags: want L of 2, got %#v", item["tags"])
	}
	if s, ok := l.Value[0].(*ddbtypes.AttributeValueMemberS); !ok || s.Value != "a" {
		t.Errorf("tags[0]: want S=a, got %#v", l.Value[0])
	}
	m, ok := item["meta"].(*ddbtypes.AttributeValueMemberM)
	if !ok {
		t.Fatalf("meta: want M, got %#v", item["meta"])
	}
	if s, ok := m.Value["city"].(*ddbtypes.AttributeValueMemberS); !ok || s.Value != "NYC" {
		t.Errorf("meta.city: want S=NYC, got %#v", m.Value["city"])
	}
	if n, ok := m.Value["zip"].(*ddbtypes.AttributeValueMemberN); !ok || n.Value != "10001" {
		t.Errorf("meta.zip: want N=10001, got %#v", m.Value["zip"])
	}
}

func TestJSONToItemErrors(t *testing.T) {
	cases := map[string]string{
		"invalid syntax": `{ "pk": }`,
		"not an object":  `["a", "b"]`,
		"empty string":   ``,
	}
	for name, src := range cases {
		if _, err := jsonToItem(src); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestConvertMaps(t *testing.T) {
	in := []map[string]interface{}{
		{"pk": "a", "sk": "1"},
		{"pk": "b", "sk": "2"},
	}
	out, err := convertMaps(in)
	if err != nil {
		t.Fatalf("convertMaps: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 items, got %d", len(out))
	}
	if s, ok := out[1]["pk"].(*ddbtypes.AttributeValueMemberS); !ok || s.Value != "b" {
		t.Errorf("out[1].pk: want S=b, got %#v", out[1]["pk"])
	}
}

func TestConvertMapsEmpty(t *testing.T) {
	out, err := convertMaps(nil)
	if err != nil {
		t.Fatalf("convertMaps(nil): %v", err)
	}
	if len(out) != 0 {
		t.Errorf("want empty, got %d", len(out))
	}
}
