package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/danielpadua/oad/internal/logging"
)

type contextKey string

const testKey contextKey = "test_value"

func testExtractor(ctx context.Context) []slog.Attr {
	if v, ok := ctx.Value(testKey).(string); ok {
		return []slog.Attr{slog.String("test_attr", v)}
	}
	return nil
}

func TestContextHandler_InjectsAttributes(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := logging.NewContextHandler(inner, testExtractor)
	logger := slog.New(handler)

	ctx := context.WithValue(context.Background(), testKey, "hello")
	logger.InfoContext(ctx, "test message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["test_attr"] != "hello" {
		t.Errorf("expected test_attr=hello, got %v", entry["test_attr"])
	}
	if entry["msg"] != "test message" {
		t.Errorf("expected msg=test message, got %v", entry["msg"])
	}
}

func TestContextHandler_OmitsWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := logging.NewContextHandler(inner, testExtractor)
	logger := slog.New(handler)

	logger.InfoContext(context.Background(), "no context value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if _, exists := entry["test_attr"]; exists {
		t.Error("expected test_attr to be absent when context has no value")
	}
}

func TestContextHandler_WithAttrsPreservesExtractors(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := logging.NewContextHandler(inner, testExtractor)
	logger := slog.New(handler.WithAttrs([]slog.Attr{slog.String("static", "value")}))

	ctx := context.WithValue(context.Background(), testKey, "dynamic")
	logger.InfoContext(ctx, "combined")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["static"] != "value" {
		t.Errorf("expected static=value, got %v", entry["static"])
	}
	if entry["test_attr"] != "dynamic" {
		t.Errorf("expected test_attr=dynamic, got %v", entry["test_attr"])
	}
}

func TestContextHandler_WithGroupPreservesExtractors(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := logging.NewContextHandler(inner, testExtractor)
	logger := slog.New(handler.WithGroup("grp"))

	ctx := context.WithValue(context.Background(), testKey, "grouped")
	logger.InfoContext(ctx, "group test", "extra", "val")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	grp, ok := entry["grp"].(map[string]any)
	if !ok {
		t.Fatalf("expected grp group in log entry, got %v", entry)
	}
	if grp["test_attr"] != "grouped" {
		t.Errorf("expected grp.test_attr=grouped, got %v", grp["test_attr"])
	}
}
