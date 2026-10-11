package speckit

import "testing"

func TestBuilders(t *testing.T) {
	for _, s := range []interface{ Check() error }{
		Range("iris", 0, 1, 0.01, ""), Enum("iso", Opts("100", "200")),
		Toggle("record"), Action("autoFocus"), ReadOnly(Toggle("tally")),
	} {
		if err := s.Check(); err != nil {
			t.Error(err)
		}
	}
	if o := Numbered(12); len(o) != 12 || o[11].Value != "12" {
		t.Errorf("Numbered(12) = %v", o)
	}
}
