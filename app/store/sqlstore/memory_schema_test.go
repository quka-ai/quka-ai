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
		"CONSTRAINT chk_quka_memory_binding_user_id CHECK (user_id <> '')",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("memory schema does not contain %q", want)
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
