package cameras_test

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestRegisterListsEveryModuleInOrder(t *testing.T) {
	r := core.NewRegistry()
	if err := cameras.Register(r, nil); err != nil {
		t.Fatal(err)
	}
	models := r.Models()
	if len(models) != 16 {
		t.Fatalf("%d modules, want 15 cameras and the demo", len(models))
	}
	if models[0].ID != "bm-pocket-4k" || models[5].ID != "bm-studio-6k-pro" || models[15].ID != "demo" {
		t.Errorf("order: first %s, sixth %s, last %s", models[0].ID, models[5].ID, models[15].ID)
	}
}

func TestRegisterTwiceFails(t *testing.T) {
	r := core.NewRegistry()
	_ = cameras.Register(r, nil)
	if err := cameras.Register(r, nil); err == nil {
		t.Error("second Register must fail with duplicates")
	}
}

func TestEveryFormerIDStillResolves(t *testing.T) {
	r := core.NewRegistry()
	_ = cameras.Register(r, nil)
	for old, cur := range map[core.ModelID]core.ModelID{
		"blackmagic.pocket4k":    "bm-pocket-4k",
		"blackmagic.studio6kpro": "bm-studio-6k-pro",
		"sony.fx6":               "sony-fx6",
		"panasonic.awue150":      "panasonic-aw-ue150",
		"demo.camera":            "demo",
	} {
		if info, ok := r.Model(old); !ok || info.ID != cur {
			t.Errorf("Model(%s) = %s, %t; want %s", old, info.ID, ok, cur)
		}
	}
}
