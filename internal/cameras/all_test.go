package cameras

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestRegister(t *testing.T) {
	r := core.NewRegistry()
	if err := Register(r); err != nil {
		t.Fatal(err)
	}
	models := r.Models()
	if len(models) != 15 {
		t.Fatalf("%d models, want 15", len(models))
	}
	if models[0].ID != "blackmagic.pocket4k" {
		t.Errorf("first model %s", models[0].ID)
	}
	if err := Register(r); err == nil {
		t.Error("second Register must fail with duplicates")
	}
}
