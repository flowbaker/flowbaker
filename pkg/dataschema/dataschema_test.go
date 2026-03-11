package dataschema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type simpleOutput struct {
	ChannelID string `json:"channel_id"`
	Timestamp string `json:"timestamp"`
}

type nestedOutput struct {
	Status  int            `json:"status"`
	Headers map[string]any `json:"headers"`
	Body    struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"body"`
}

func TestFromStruct(t *testing.T) {
	d, err := FromStruct(simpleOutput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.IsEmpty() {
		t.Fatal("expected non-empty schema")
	}

	raw, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	if _, ok := props["channel_id"]; !ok {
		t.Error("expected channel_id property")
	}
	if _, ok := props["timestamp"]; !ok {
		t.Error("expected timestamp property")
	}
}

func TestFromStructPointer(t *testing.T) {
	d, err := FromStruct(&simpleOutput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.IsEmpty() {
		t.Fatal("expected non-empty schema")
	}
}

func TestFromStructNested(t *testing.T) {
	d, err := FromStruct(nestedOutput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	for _, key := range []string{"status", "headers", "body"} {
		if _, ok := props[key]; !ok {
			t.Errorf("expected %s property", key)
		}
	}
}

func TestFromString(t *testing.T) {
	input := `{"type":"object","properties":{"name":{"type":"string"}}}`

	d, err := FromString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.IsEmpty() {
		t.Fatal("expected non-empty schema")
	}

	output := d.String()
	if output == "" {
		t.Fatal("expected non-empty string")
	}
}

func TestFromBytes(t *testing.T) {
	input := []byte(`{"type":"object","properties":{"count":{"type":"integer"}}}`)

	d, err := FromBytes(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.IsEmpty() {
		t.Fatal("expected non-empty schema")
	}
}

func TestFromBytesInvalid(t *testing.T) {
	_, err := FromBytes([]byte(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")

	content := []byte(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	d, err := FromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.IsEmpty() {
		t.Fatal("expected non-empty schema")
	}
}

func TestFromFileNotFound(t *testing.T) {
	_, err := FromFile("/nonexistent/path/schema.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	original, err := FromStruct(simpleOutput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	restored := &DataSchema{}
	if err := restored.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if restored.IsEmpty() {
		t.Fatal("expected non-empty schema after unmarshal")
	}

	restoredData, err := restored.MarshalJSON()
	if err != nil {
		t.Fatalf("re-marshal error: %v", err)
	}

	var orig, rest map[string]any
	json.Unmarshal(data, &orig)
	json.Unmarshal(restoredData, &rest)

	origProps := orig["properties"].(map[string]any)
	restProps := rest["properties"].(map[string]any)

	if len(origProps) != len(restProps) {
		t.Errorf("property count mismatch: %d vs %d", len(origProps), len(restProps))
	}
}

func TestIsEmpty(t *testing.T) {
	var nilSchema *DataSchema
	if !nilSchema.IsEmpty() {
		t.Error("nil schema should be empty")
	}

	emptySchema := &DataSchema{}
	if !emptySchema.IsEmpty() {
		t.Error("zero-value schema should be empty")
	}

	populated, _ := FromString(`{"type":"object"}`)
	if populated.IsEmpty() {
		t.Error("populated schema should not be empty")
	}
}

func TestStringOutput(t *testing.T) {
	var nilSchema *DataSchema
	if nilSchema.String() != "" {
		t.Error("nil schema String() should return empty")
	}

	d, _ := FromString(`{"type":"object","properties":{"x":{"type":"string"}}}`)
	s := d.String()
	if s == "" {
		t.Fatal("expected non-empty string output")
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("String() output should be valid JSON: %v", err)
	}
}

func TestMustFromStructPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			return
		}
		t.Error("expected panic")
	}()

	MustFromStruct(make(chan int))
}

func TestMustFromStringPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			return
		}
		t.Error("expected panic")
	}()

	MustFromString(`{invalid`)
}

func TestMustFromStruct(t *testing.T) {
	d := MustFromStruct(simpleOutput{})
	if d.IsEmpty() {
		t.Fatal("expected non-empty schema")
	}
}

func TestMarshalNilSchema(t *testing.T) {
	var d *DataSchema
	data, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("expected null, got %s", string(data))
	}
}

func TestFromStructThirdPartyType(t *testing.T) {
	type externalMsg struct {
		Type      string `json:"type,omitempty"`
		Channel   string `json:"channel,omitempty"`
		User      string `json:"user,omitempty"`
		Text      string `json:"text,omitempty"`
		Timestamp string `json:"ts,omitempty"`
	}

	d, err := FromStruct(externalMsg{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, _ := d.MarshalJSON()
	var m map[string]any
	json.Unmarshal(raw, &m)

	props := m["properties"].(map[string]any)
	for _, key := range []string{"type", "channel", "user", "text", "ts"} {
		if _, ok := props[key]; !ok {
			t.Errorf("expected %s property (from json tag)", key)
		}
	}
}

type deepNested struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Value string `json:"value"`
			} `json:"level3"`
		} `json:"level2"`
	} `json:"level1"`
	Name string `json:"name"`
}

func TestMaxDepth1(t *testing.T) {
	d, err := FromStruct(deepNested{}, Options{MaxDepth: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, _ := d.MarshalJSON()
	var m map[string]any
	json.Unmarshal(raw, &m)

	props := m["properties"].(map[string]any)

	nameP := props["name"].(map[string]any)
	if nameP["type"] != "string" {
		t.Error("expected name to be string")
	}

	level1P := props["level1"].(map[string]any)
	if _, hasProps := level1P["properties"]; hasProps {
		t.Error("depth 1 should not expand level1 properties")
	}
}

func TestMaxDepth2(t *testing.T) {
	d, err := FromStruct(deepNested{}, Options{MaxDepth: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, _ := d.MarshalJSON()
	var m map[string]any
	json.Unmarshal(raw, &m)

	props := m["properties"].(map[string]any)
	level1P := props["level1"].(map[string]any)

	innerProps, ok := level1P["properties"].(map[string]any)
	if !ok {
		t.Fatal("depth 2 should expand level1 properties")
	}

	level2P := innerProps["level2"].(map[string]any)
	if _, hasProps := level2P["properties"]; hasProps {
		t.Error("depth 2 should not expand level2 properties")
	}
}

func TestMaxDepthZeroMeansUnlimited(t *testing.T) {
	d1, _ := FromStruct(deepNested{})
	d2, _ := FromStruct(deepNested{}, Options{MaxDepth: 0})

	r1, _ := d1.MarshalJSON()
	r2, _ := d2.MarshalJSON()

	var m1, m2 map[string]any
	json.Unmarshal(r1, &m1)
	json.Unmarshal(r2, &m2)

	p1 := m1["properties"].(map[string]any)
	p2 := m2["properties"].(map[string]any)

	l1a := p1["level1"].(map[string]any)
	l1b := p2["level1"].(map[string]any)

	_, a := l1a["properties"]
	_, b := l1b["properties"]
	if a != b {
		t.Error("MaxDepth 0 should behave same as no options")
	}
}

func TestSchemaAccessor(t *testing.T) {
	d, _ := FromStruct(simpleOutput{})
	if d.Schema() == nil {
		t.Error("expected non-nil schema")
	}

	var nilSchema *DataSchema
	if nilSchema.Schema() != nil {
		t.Error("expected nil schema from nil DataSchema")
	}
}
