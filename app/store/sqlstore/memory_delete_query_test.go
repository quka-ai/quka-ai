package sqlstore

import (
	"reflect"
	"strings"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestDeleteMemoryBindingByMemoryIDsQueryGlobalMemory(t *testing.T) {
	query := deleteMemoryBindingByMemoryIDsQuery(types.TABLE_MEMORY_BINDING.Name(), types.GLOBAL_MEMORY_SPACE_ID, []string{"mem_1", "mem_2"})
	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql() error = %v", err)
	}
	if strings.Contains(sql, "space_id") {
		t.Fatalf("global memory binding cleanup should not be space-scoped: %s", sql)
	}
	if want := []any{"mem_1", "mem_2"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestDeleteMemoryBindingByMemoryIDsQuerySpaceMemory(t *testing.T) {
	query := deleteMemoryBindingByMemoryIDsQuery(types.TABLE_MEMORY_BINDING.Name(), "space_1", []string{"mem_1", "mem_2"})
	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql() error = %v", err)
	}
	if !strings.Contains(sql, "space_id =") {
		t.Fatalf("space memory binding cleanup should be space-scoped: %s", sql)
	}
	if want := []any{"mem_1", "mem_2", "space_1"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestDeleteMemoryEdgeByMemoryIDsQueryGlobalMemory(t *testing.T) {
	query := deleteMemoryEdgeByMemoryIDsQuery(types.TABLE_MEMORY_EDGE.Name(), types.GLOBAL_MEMORY_SPACE_ID, []string{"mem_1", "mem_2"})
	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql() error = %v", err)
	}
	if strings.Contains(sql, "space_id") {
		t.Fatalf("global memory edge cleanup should not be space-scoped: %s", sql)
	}
	if want := []any{"mem_1", "mem_2", "mem_1", "mem_2"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestDeleteMemoryEdgeByMemoryIDsQuerySpaceMemory(t *testing.T) {
	query := deleteMemoryEdgeByMemoryIDsQuery(types.TABLE_MEMORY_EDGE.Name(), "space_1", []string{"mem_1", "mem_2"})
	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql() error = %v", err)
	}
	if !strings.Contains(sql, "space_id =") {
		t.Fatalf("space memory edge cleanup should be space-scoped: %s", sql)
	}
	if want := []any{"mem_1", "mem_2", "mem_1", "mem_2", "space_1"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}
