package sqlstore

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type PGConfig struct {
	DSN string `toml:"dsn"`
}

func (m *PGConfig) FromENV() {
	m.DSN = os.Getenv("BREW_API_POSTGRESQL_DSN")
}

func (m PGConfig) FormatDSN() string {
	return m.DSN
}

func TestQuery(t *testing.T) {
	cfg := PGConfig{}
	cfg.FromENV()
	provider := MustSetup(cfg)()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	f, err := os.Open("./test_vectors")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	var vectors []float32
	if err = json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	res, err := provider.stores.VectorStore.Query(ctx, types.GetVectorsOptions{SpaceID: "test"}, pgvector.NewVector(vectors), 5)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}
