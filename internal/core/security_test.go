package core

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestValidateConfigRejectsUnsafeText(t *testing.T) {
	m := testModel("acme.cam")
	for name, text := range map[string]string{
		"bidi override": "Cam‮A",
		"invalid utf8":  "Cam\xff",
		"C1 control":    "Cam\u0085",
		"zero width":    "Cam​",
	} {
		c := CameraConfig{Model: m.ID, Host: "10.0.0.1", Name: text}
		if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("name %s: ValidateConfig = %v, want ErrInvalidConfig", name, err)
		}
		c = CameraConfig{Model: m.ID, Host: "10.0.0.1", Username: text}
		if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("username %s: ValidateConfig = %v, want ErrInvalidConfig", name, err)
		}
	}
	ok := CameraConfig{Model: m.ID, Host: "10.0.0.1", Name: "Caméra 1 — plateau", Password: "pässwörd!"}
	if err := ValidateConfig(m, ok); err != nil {
		t.Fatalf("ValidateConfig(accented name) = %v, want nil", err)
	}
}

func TestValidateConfigErrorNeverEchoesPassword(t *testing.T) {
	m := testModel("acme.cam")
	c := CameraConfig{Model: m.ID, Host: "10.0.0.1", Password: "hunter2\x01" + strings.Repeat("x", 200)}
	err := ValidateConfig(m, c)
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("ValidateConfig = %v", err)
	}
}

func TestCameraConfigRedactsPasswordInLogsAndFormatting(t *testing.T) {
	c := CameraConfig{Model: "acme.cam", Host: "10.0.0.1", Username: "admin", Password: "hunter2"}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		if s := fmt.Sprintf(verb, c); strings.Contains(s, "hunter2") {
			t.Errorf("fmt %s leaked the password: %s", verb, s)
		}
		if s := fmt.Sprintf(verb, &c); strings.Contains(s, "hunter2") {
			t.Errorf("fmt %s (pointer) leaked the password: %s", verb, s)
		}
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("cfg", "cfg", c, "ptr", &c)
	slog.New(slog.NewTextHandler(&buf, nil)).Info("cfg", "cfg", c)
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("slog leaked the password: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "10.0.0.1") {
		t.Fatalf("redacted log lost the useful fields: %s", buf.String())
	}
}

func TestCheckControlID(t *testing.T) {
	for _, id := range []ControlID{"iris", "sony.slowShutter", "gain_R-2"} {
		if err := CheckControlID(id); err != nil {
			t.Errorf("CheckControlID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range []ControlID{"", ControlID(strings.Repeat("a", MaxIDLen+1)), "iris\n", "a b", "<script>", "ïris"} {
		err := CheckControlID(id)
		if !errors.Is(err, ErrUnknownControl) {
			t.Errorf("CheckControlID(%q) = %v, want ErrUnknownControl", id, err)
		}
		if err != nil && len(err.Error()) > 100 {
			t.Errorf("CheckControlID error echoes the input: %d bytes", len(err.Error()))
		}
	}
}

func TestValueCheck(t *testing.T) {
	for _, v := range []Value{Number(1), Choice("1/50"), Bool(true), Trigger()} {
		if err := v.Check(); err != nil {
			t.Errorf("%+v.Check() = %v, want nil", v, err)
		}
	}
	for name, v := range map[string]Value{
		"long choice":  Choice(strings.Repeat("x", MaxIDLen+1)),
		"control char": Choice("a\x00b"),
		"invalid utf8": Choice("\xff"),
		"unknown kind": {Kind: "evil"},
	} {
		err := v.Check()
		if !errors.Is(err, ErrInvalidValue) {
			t.Errorf("%s: Check() = %v, want ErrInvalidValue", name, err)
		}
		if err != nil && len(err.Error()) > 100 {
			t.Errorf("%s: error echoes the input: %d bytes", name, len(err.Error()))
		}
	}
}

func TestSpecCheckRejectsOversizedOption(t *testing.T) {
	s := ControlSpec{ID: "nd", Kind: KindEnum, Options: []Option{{Value: strings.Repeat("x", MaxIDLen+1)}}}
	if s.Check() == nil {
		t.Fatal("Check accepted an option value longer than MaxIDLen")
	}
}
