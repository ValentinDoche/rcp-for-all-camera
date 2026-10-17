package i18n

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

var placeholderRe = regexp.MustCompile(`\{[A-Za-z0-9]+\}`)

func placeholders(s string) string {
	m := placeholderRe.FindAllString(s, -1)
	sort.Strings(m)
	return strings.Join(m, ",")
}

func TestEveryKeyIsInEveryLocale(t *testing.T) {
	b := Default()
	langs := b.Languages()
	if len(langs) < 2 {
		t.Fatalf("expected fr and en, got %v", langs)
	}
	for _, a := range langs {
		for _, o := range langs {
			for k := range b.msgs[a.Code] {
				if _, ok := b.msgs[o.Code][k]; !ok {
					t.Errorf("key %q is in %s but not in %s", k, a.Code, o.Code)
				}
			}
		}
	}
}

func TestValuesNotEmptyAndSamePlaceholders(t *testing.T) {
	b := Default()
	for k, ref := range b.msgs["en"] {
		for _, l := range b.Languages() {
			v := b.msgs[l.Code][k]
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: empty value for %q", l.Code, k)
			}
			if placeholders(v) != placeholders(ref) {
				t.Errorf("%s: placeholders of %q differ from en: %q vs %q", l.Code, k, v, ref)
			}
		}
	}
}

func TestNoDashesInAnyValue(t *testing.T) {
	for code, m := range Default().msgs {
		for k, v := range m {
			if strings.ContainsAny(v, "—–") {
				t.Errorf("%s: dash in %q: %q", code, k, v)
			}
		}
	}
}

func TestEveryLocaleHasName(t *testing.T) {
	for _, l := range Default().Languages() {
		if l.Name == "" || l.Code == "" {
			t.Errorf("language %+v incomplete", l)
		}
	}
	if Default().Translator("fr").Name() != "Français" {
		t.Error("fr name")
	}
}

func TestFallbackChain(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/en.json": {Data: []byte(`{"_meta.name":"English","_meta.decimal":".","a":"A en","b":"B en {x}"}`)},
		"locales/de.json": {Data: []byte(`{"_meta.name":"Deutsch","_meta.decimal":",","a":"A de"}`)},
	}
	b, err := Load(fsys, "locales")
	if err != nil {
		t.Fatal(err)
	}
	de := b.Translator("de")
	if de.T("a") != "A de" {
		t.Errorf("own string = %q", de.T("a"))
	}
	if got := de.T("b", "x", 3); got != "B en 3" {
		t.Errorf("falls back to en = %q", got)
	}
	if got := de.T("zzz"); got != "zzz" {
		t.Errorf("falls back to key = %q", got)
	}
	if b.Translator("xx").Lang() != "en" {
		t.Errorf("unknown language should give en")
	}
	if de.Number(1.5, 1) != "1,5" || b.Translator("en").Number(1.5, 1) != "1.5" {
		t.Error("decimal mark from _meta.decimal")
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	if _, err := Load(fstest.MapFS{"l/en.json": {Data: []byte(`{`)}}, "l"); err == nil {
		t.Error("bad JSON accepted")
	}
	if _, err := Load(fstest.MapFS{"l/fr.json": {Data: []byte(`{"_meta.name":"F"}`)}}, "l"); err == nil {
		t.Error("missing en accepted")
	}
}

func TestPlaceholders(t *testing.T) {
	tr := Default().Translator("en")
	if got := tr.T("config.saved", "slot", 3); got != "Cam3 saved" {
		t.Errorf("T = %q", got)
	}
	// unfilled placeholders stay visible, an odd trailing argument is ignored
	if got := tr.T("config.saved", "oops"); got != "Cam{slot} saved" {
		t.Errorf("T = %q", got)
	}
	if got := Default().Translator("fr").T("details.title", "slot", 2); got != "Cam2 : détails" {
		t.Errorf("fr T = %q", got)
	}
}

func TestFormatPerLanguage(t *testing.T) {
	fr, en := Default().Translator("fr"), Default().Translator("en")
	if fr.Number(0.5, 2) != "0,50" || en.Number(0.5, 2) != "0.50" {
		t.Error("Number")
	}
	if fr.Number(-0.001, 2) != "0,00" {
		t.Errorf("negative zero = %q", fr.Number(-0.001, 2))
	}
	cases := []struct {
		bps    float64
		fr, en string
	}{
		{0, "0 o/s", "0 B/s"}, {512, "512 o/s", "512 B/s"}, {5329.3, "5,2 ko/s", "5.2 KB/s"},
		{1_572_864, "1,5 Mo/s", "1.5 MB/s"}, {3.2e9, "3,0 Go/s", "3.0 GB/s"},
	}
	for _, c := range cases {
		if fr.Rate(c.bps) != c.fr || en.Rate(c.bps) != c.en {
			t.Errorf("Rate(%v) = %q / %q", c.bps, fr.Rate(c.bps), en.Rate(c.bps))
		}
	}
	if got := fr.Memory(23915626496, 67939266560); got != "22,3 / 63,3 Go" {
		t.Errorf("fr memory = %q", got)
	}
	if got := en.Memory(23915626496, 67939266560); got != "22.3 / 63.3 GB" {
		t.Errorf("en memory = %q", got)
	}
}

// docs/i18n.md explains how to add a string or a language.
func TestDocExists(t *testing.T) {
	if _, err := os.Stat("../../docs/i18n.md"); err != nil {
		t.Error(err)
	}
}
