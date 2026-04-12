// Package validation provides a reusable JSON Schema validation engine
// built on santhosh-tekuri/jsonschema. It compiles JSON Schema documents
// into validators that can be applied to instances, and integrates with
// apierr to produce consistent API error responses.
package validation

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/danielpadua/oad/internal/apierr"
)

// Compile parses and compiles a JSON Schema document into a reusable Validator.
// The schemaURI is used as the resource identifier (e.g., "oad://entity-type/user").
func Compile(schemaURI string, schema json.RawMessage) (*Validator, error) {
	var doc any
	if err := json.Unmarshal(schema, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON in schema: %w", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURI, doc); err != nil {
		return nil, fmt.Errorf("adding schema resource %s: %w", schemaURI, err)
	}

	compiled, err := c.Compile(schemaURI)
	if err != nil {
		return nil, fmt.Errorf("compiling schema %s: %w", schemaURI, err)
	}

	return &Validator{schema: compiled}, nil
}

// Validator validates JSON instances against a compiled schema.
type Validator struct {
	schema *jsonschema.Schema
}

// Validate checks the instance against the compiled schema.
// Returns nil if valid, or an *apierr.APIError with VALIDATION_FAILED code
// containing human-readable details for each violation.
func (v *Validator) Validate(instance any) *apierr.APIError {
	err := v.schema.Validate(instance)
	if err == nil {
		return nil
	}

	validationErr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return apierr.BadRequest("schema validation failed: " + err.Error())
	}

	details := flattenValidationErrors(validationErr)
	return apierr.ValidationFailed(details...)
}

// ValidateRaw unmarshals a JSON byte slice and validates it against the schema.
func (v *Validator) ValidateRaw(data json.RawMessage) *apierr.APIError {
	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		return apierr.BadRequest("invalid JSON: " + err.Error())
	}
	return v.Validate(instance)
}

// ValidateIsJSONSchema checks that the given document is itself a valid
// JSON Schema. Used for validating entity_type_definition.allowed_properties
// and system_overlay_schema.allowed_overlay_properties at creation/update time.
func ValidateIsJSONSchema(schema json.RawMessage) *apierr.APIError {
	var doc any
	if err := json.Unmarshal(schema, &doc); err != nil {
		return apierr.BadRequest("invalid JSON: " + err.Error())
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("validate://input", doc); err != nil {
		return apierr.ValidationFailed("invalid JSON Schema: " + err.Error())
	}

	if _, err := c.Compile("validate://input"); err != nil {
		return apierr.ValidationFailed("invalid JSON Schema: " + err.Error())
	}

	return nil
}

// flattenValidationErrors recursively extracts human-readable messages from
// the jsonschema validation error tree.
func flattenValidationErrors(err *jsonschema.ValidationError) []string {
	var details []string
	collectErrors(err, &details)
	return details
}

func collectErrors(err *jsonschema.ValidationError, details *[]string) {
	if len(err.Causes) == 0 {
		msg := err.Error()
		if msg != "" {
			*details = append(*details, msg)
		}
		return
	}
	for _, cause := range err.Causes {
		collectErrors(cause, details)
	}
}
