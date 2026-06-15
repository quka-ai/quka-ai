package sqlstore

import (
	"os"
	"strings"
	"testing"
)

func TestMemorySchemaEnforcesCenterHostedLayerConstraints(t *testing.T) {
	schema, err := os.ReadFile("memory.sql")
	if err != nil {
		t.Fatalf("ReadFile(memory.sql) error = %v", err)
	}
	bindingSchema, err := os.ReadFile("memory_binding.sql")
	if err != nil {
		t.Fatalf("ReadFile(memory_binding.sql) error = %v", err)
	}
	raw := string(schema) + "\n" + string(bindingSchema)

	for _, want := range []string{
		"CONSTRAINT chk_quka_memory_layer_scope CHECK",
		"(scope = 'user' AND user_id <> '')",
		"OR (scope = 'space' AND space_id <> '-' AND space_id <> '')",
		"CONSTRAINT chk_quka_memory_content_backing CHECK",
		"OR (memory_type = 'working' AND content <> '')",
		"WHERE knowledge_id <> ''",
		"CONSTRAINT chk_quka_memory_binding_user_id CHECK (user_id <> '')",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("memory schema does not contain %q", want)
		}
	}
}

func TestWorkingMemoryInlineContentMigration(t *testing.T) {
	migration, err := os.ReadFile("migrations/working_memory_inline_content.sql")
	if err != nil {
		t.Fatalf("ReadFile(working_memory_inline_content.sql) error = %v", err)
	}
	raw := string(migration)

	for _, want := range []string{
		"ADD COLUMN IF NOT EXISTS title",
		"ADD COLUMN IF NOT EXISTS content",
		"ADD COLUMN IF NOT EXISTS content_type",
		"ALTER COLUMN knowledge_id SET DEFAULT ''",
		"WHERE knowledge_id <> ''",
		"ADD CONSTRAINT chk_quka_memory_content_backing CHECK",
		") NOT VALID;",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("working memory inline content migration does not contain %q", want)
		}
	}
}

func TestMemoryLayerConstraintMigrationUsesNotValid(t *testing.T) {
	migration, err := os.ReadFile("migrations/memory_layer_constraints.sql")
	if err != nil {
		t.Fatalf("ReadFile(memory_layer_constraints.sql) error = %v", err)
	}
	raw := string(migration)

	for _, want := range []string{
		"ADD CONSTRAINT chk_quka_memory_layer_scope CHECK",
		") NOT VALID;",
		"ADD CONSTRAINT chk_quka_memory_binding_user_id CHECK (user_id <> '') NOT VALID;",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("memory layer constraint migration does not contain %q", want)
		}
	}
}

func TestMemoryBindingUserIsolationMigrationRebuildsIndexes(t *testing.T) {
	migration, err := os.ReadFile("migrations/memory_binding_user_isolation.sql")
	if err != nil {
		t.Fatalf("ReadFile(memory_binding_user_isolation.sql) error = %v", err)
	}
	raw := string(migration)

	for _, want := range []string{
		"DROP INDEX IF EXISTS idx_quka_memory_binding_user_context;",
		"DROP INDEX IF EXISTS idx_quka_memory_binding_user_memory;",
		"DROP INDEX IF EXISTS idx_quka_memory_binding_unique_context_memory;",
		"ON quka_memory_binding (space_id, user_id, memory_id, context_type, context_id, binding_type);",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("memory binding user isolation migration does not contain %q", want)
		}
	}
}

func TestFixedPinSchemaUsesUserSpaceIsolation(t *testing.T) {
	schema, err := os.ReadFile("fixed_pin.sql")
	if err != nil {
		t.Fatalf("ReadFile(fixed_pin.sql) error = %v", err)
	}
	raw := string(schema)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS quka_fixed_pin",
		"content TEXT NOT NULL",
		"content_type VARCHAR(30) NOT NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_fixed_pin_space_user ON quka_fixed_pin (space_id, user_id)",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("fixed pin schema does not contain %q", want)
		}
	}
}
