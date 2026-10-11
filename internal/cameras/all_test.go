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
	if len(models) != 16 {
		t.Fatalf("%d models, want 15 cameras and the demo", len(models))
	}
	if models[0].ID != "blackmagic.pocket4k" || models[15].ID != "demo.camera" {
		t.Errorf("first model %s, last %s", models[0].ID, models[15].ID)
	}
	if err := Register(r); err == nil {
		t.Error("second Register must fail with duplicates")
	}
}
