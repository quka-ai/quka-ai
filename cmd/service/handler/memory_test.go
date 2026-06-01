package handler

import (
	"reflect"
	"strings"
	"testing"
)

func TestHydrateMemoryRequestRuntimeContextIsOptional(t *testing.T) {
	field, ok := reflect.TypeOf(HydrateMemoryRequest{}).FieldByName("RuntimeContext")
	if !ok {
		t.Fatal("HydrateMemoryRequest.RuntimeContext field not found")
	}
	if binding := field.Tag.Get("binding"); strings.Contains(binding, "required") {
		t.Fatalf("RuntimeContext binding tag = %q, want optional runtime_context for plain recall hydration", binding)
	}
}
