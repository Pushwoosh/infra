package infralog

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func fieldNames(fields []zap.Field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Key
	}
	return names
}

func TestFieldsFromEmptyContext(t *testing.T) {
	fields := fieldsFromContext(context.Background())
	if fields != nil {
		t.Errorf("expected nil, got %v", fields)
	}
}

func TestWithField(t *testing.T) {
	ctx := WithField(context.Background(), zap.String("a", "1"))
	fields := fieldsFromContext(ctx)
	if len(fields) != 1 || fields[0].Key != "a" {
		t.Errorf("expected [a], got %v", fieldNames(fields))
	}
}

func TestWithFields(t *testing.T) {
	ctx := WithFields(context.Background(), zap.String("a", "1"), zap.String("b", "2"))
	fields := fieldsFromContext(ctx)
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].Key != "a" || fields[1].Key != "b" {
		t.Errorf("expected [a b], got %v", fieldNames(fields))
	}
}

func TestWithFieldsEmpty(t *testing.T) {
	ctx := WithFields(context.Background())
	if ctx != context.Background() {
		t.Error("expected same context when no fields provided")
	}
}

// Fields from all ancestor contexts must be collected in root-to-leaf order.
func TestFieldsAccumulateAcrossChain(t *testing.T) {
	ctx0 := context.Background()
	ctx1 := WithField(ctx0, zap.String("app", "myapp"))
	ctx2 := WithField(ctx1, zap.String("req_id", "123"))
	ctx3 := WithField(ctx2, zap.String("user_id", "456"))

	fields := fieldsFromContext(ctx3)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[0].Key != "app" || fields[1].Key != "req_id" || fields[2].Key != "user_id" {
		t.Errorf("expected [app req_id user_id], got %v", fieldNames(fields))
	}
}

// Each goroutine must be able to add fields to a shared parent context
// without affecting other goroutines or the parent itself.
func TestWithFieldDoesNotMutateParent(t *testing.T) {
	rootCtx := WithField(context.Background(), zap.String("app", "myapp"))

	// two goroutines add their own fields to the same parent context
	ctx1 := WithField(rootCtx, zap.String("goroutine", "1"))
	ctx2 := WithField(rootCtx, zap.String("goroutine", "2"))

	// each sees only its own fields plus the parent's
	fields1 := fieldsFromContext(ctx1)
	fields2 := fieldsFromContext(ctx2)

	if len(fields1) != 2 || fields1[1].Key != "goroutine" || fields1[1].String != "1" {
		t.Errorf("ctx1: expected [app goroutine=1], got %v", fieldNames(fields1))
	}
	if len(fields2) != 2 || fields2[1].Key != "goroutine" || fields2[1].String != "2" {
		t.Errorf("ctx2: expected [app goroutine=2], got %v", fieldNames(fields2))
	}

	// parent context must be untouched
	rootFields := fieldsFromContext(rootCtx)
	if len(rootFields) != 1 || rootFields[0].Key != "app" {
		t.Errorf("rootCtx: expected [app], got %v", fieldNames(rootFields))
	}
}

// WithField must always return a new context, never the same one.
func TestWithFieldReturnsNewContext(t *testing.T) {
	ctx0 := context.Background()
	ctx1 := WithField(ctx0, zap.String("a", "1"))
	ctx2 := WithField(ctx1, zap.String("b", "2"))

	if ctx1 == ctx0 {
		t.Error("WithField should return a new context, not the same")
	}
	if ctx2 == ctx1 {
		t.Error("WithField should return a new context, not the same")
	}
}

// Fields must be returned in root-to-leaf order.
func TestFieldsOrder(t *testing.T) {
	ctx := context.Background()
	ctx = WithFields(ctx, zap.String("1", ""), zap.String("2", ""))
	ctx = WithFields(ctx, zap.String("3", ""), zap.String("4", ""))
	ctx = WithField(ctx, zap.String("5", ""))

	fields := fieldsFromContext(ctx)
	if len(fields) != 5 {
		t.Fatalf("expected 5 fields, got %d", len(fields))
	}
	for i, f := range fields {
		expected := string(rune('1' + i))
		if f.Key != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, f.Key)
		}
	}
}
