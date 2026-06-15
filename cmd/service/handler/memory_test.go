package handler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
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

func TestUpdateMemoryRequestSupportsInlineWorkingContent(t *testing.T) {
	reqType := reflect.TypeOf(UpdateMemoryRequest{})
	for _, name := range []string{"Title", "Content", "ContentType"} {
		if _, ok := reqType.FieldByName(name); !ok {
			t.Fatalf("UpdateMemoryRequest.%s field not found", name)
		}
	}

	contentField, _ := reqType.FieldByName("Content")
	if contentField.Type != reflect.TypeOf((*types.KnowledgeContent)(nil)) {
		t.Fatalf("Content field type = %v, want *types.KnowledgeContent", contentField.Type)
	}
}
