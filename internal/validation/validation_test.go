package validation_test

import (
	"encoding/json"
	"testing"

	"github.com/danielpadua/oad/internal/validation"
)

func TestCompile_ValidSchema(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"]
	}`)

	v, err := validation.Compile("test://schema", schema)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestCompile_InvalidJSON(t *testing.T) {
	schema := json.RawMessage(`{not valid json}`)
	_, err := validation.Compile("test://bad", schema)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidator_ValidInstance(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"required": ["name"]
	}`)

	v, err := validation.Compile("test://schema", schema)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	instance := map[string]any{"name": "Alice"}
	apiErr := v.Validate(instance)
	if apiErr != nil {
		t.Errorf("expected no error, got: %v", apiErr)
	}
}

func TestValidator_InvalidInstance(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"]
	}`)

	v, err := validation.Compile("test://schema", schema)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Missing required field "name"
	instance := map[string]any{"age": 25}
	apiErr := v.Validate(instance)
	if apiErr == nil {
		t.Fatal("expected validation error")
	}
	if apiErr.Code != "VALIDATION_FAILED" {
		t.Errorf("expected code VALIDATION_FAILED, got %s", apiErr.Code)
	}
	if len(apiErr.Details) == 0 {
		t.Error("expected non-empty error details")
	}
}

func TestValidator_WrongType(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"age": {"type": "integer"}
		}
	}`)

	v, err := validation.Compile("test://schema", schema)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	instance := map[string]any{"age": "not-a-number"}
	apiErr := v.Validate(instance)
	if apiErr == nil {
		t.Fatal("expected validation error for wrong type")
	}
}

func TestValidateRaw_Valid(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"x": {"type": "number"}}}`)
	v, err := validation.Compile("test://raw", schema)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	apiErr := v.ValidateRaw(json.RawMessage(`{"x": 42}`))
	if apiErr != nil {
		t.Errorf("expected no error, got: %v", apiErr)
	}
}

func TestValidateRaw_InvalidJSON(t *testing.T) {
	schema := json.RawMessage(`{"type": "object"}`)
	v, err := validation.Compile("test://raw", schema)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	apiErr := v.ValidateRaw(json.RawMessage(`{broken`))
	if apiErr == nil {
		t.Fatal("expected error for invalid JSON input")
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected code BAD_REQUEST, got %s", apiErr.Code)
	}
}

func TestValidateIsJSONSchema_Valid(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {"name": {"type": "string"}}
	}`)

	apiErr := validation.ValidateIsJSONSchema(schema)
	if apiErr != nil {
		t.Errorf("expected valid schema, got: %v", apiErr)
	}
}

func TestValidateIsJSONSchema_InvalidJSON(t *testing.T) {
	apiErr := validation.ValidateIsJSONSchema(json.RawMessage(`{broken}`))
	if apiErr == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateIsJSONSchema_BooleanSchemaTrue(t *testing.T) {
	apiErr := validation.ValidateIsJSONSchema(json.RawMessage(`true`))
	if apiErr != nil {
		t.Errorf("expected boolean true schema to be valid, got: %v", apiErr)
	}
}
