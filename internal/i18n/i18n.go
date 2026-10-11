// Package i18n translates the application's strings. Each language is one
// JSON file under locales/ (flat dotted keys, "{name}" placeholders), embedded
// in the binary. It has no UI dependency, so any front end can use it.
//
// Lookup order for a key: the requested language, then English, then the key
// itself, so a missing string is visible but never fatal.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed locales/*.json
var embedded embed.FS

// FallbackLang is the language used when a string is missing.
const FallbackLang = "en"

// Keys read from every locale file besides the UI strings.
const (
	keyName    = "_meta.name"    // language name in its own language
	keyDecimal = "_meta.decimal" // decimal mark
)

// Lang describes one available language.
type Lang struct {
	Code string // file name without .json, e.g. "fr"
	Name string // _meta.name, e.g. "Français"
}

// Bundle holds every loaded locale.
type Bundle struct {
	langs []Lang
	msgs  map[string]map[string]string
}

var (
	defOnce sync.Once
	defB    *Bundle
)

// Default returns the bundle of the embedded locales. A broken embedded file
// is a programming error caught by the tests, so it panics.
func Default() *Bundle {
	defOnce.Do(func() {
		b, err := Load(embedded, "locales")
		if err != nil {
			panic(err)
		}
		defB = b
	})
	return defB
}

// Load reads every <code>.json file of dir in fsys. English is required: it
// is the fallback language.
func Load(fsys fs.FS, dir string) (*Bundle, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("i18n: read %s: %w", dir, err)
	}
	b := &Bundle{msgs: map[string]map[string]string{}}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		code := strings.TrimSuffix(name, ".json")
		data, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", name, err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", name, err)
		}
		b.msgs[code] = m
		n := m[keyName]
		if n == "" {
			n = code
		}
		b.langs = append(b.langs, Lang{Code: code, Name: n})
	}
	if _, ok := b.msgs[FallbackLang]; !ok {
		return nil, fmt.Errorf("i18n: %s.json is required (fallback language)", FallbackLang)
	}
	sort.Slice(b.langs, func(i, j int) bool { return b.langs[i].Code < b.langs[j].Code })
	return b, nil
}

// Languages lists the loaded languages, sorted by code.
func (b *Bundle) Languages() []Lang { return append([]Lang(nil), b.langs...) }

// Has reports whether a language is loaded.
func (b *Bundle) Has(code string) bool { _, ok := b.msgs[code]; return ok }

// Translator returns the translator for a language; an unknown code gives
// the fallback language.
func (b *Bundle) Translator(code string) *Translator {
	if !b.Has(code) {
		code = FallbackLang
	}
	return &Translator{b: b, code: code}
}

// Translator translates and formats for one language. It is immutable:
// switching language means using another Translator.
type Translator struct {
	b    *Bundle
	code string
}

// Lang returns the language code.
func (t *Translator) Lang() string { return t.code }

// Name returns the language's own name.
func (t *Translator) Name() string { return t.lookup(keyName, t.code) }

func (t *Translator) lookup(key, def string) string {
	if v := t.b.msgs[t.code][key]; v != "" {
		return v
	}
	if v := t.b.msgs[FallbackLang][key]; v != "" {
		return v
	}
	return def
}

// T returns the string for key with "{name}" placeholders replaced by the
// name/value pairs in args: T("config.saved", "slot", 3). Placeholders without
// a value stay as written.
func (t *Translator) T(key string, args ...any) string {
	s := t.lookup(key, key)
	if len(args) < 2 || !strings.Contains(s, "{") {
		return s
	}
	pairs := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		pairs = append(pairs, "{"+fmt.Sprint(args[i])+"}", fmt.Sprint(args[i+1]))
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// Number formats v with the language's decimal mark.
func (t *Translator) Number(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if f, err := strconv.ParseFloat(s, 64); err == nil && f == 0 {
		s = strconv.FormatFloat(0, 'f', decimals, 64) // no "-0,00"
	}
	return strings.Replace(s, ".", t.lookup(keyDecimal, "."), 1)
}

// Rate formats a byte rate with the language's units (binary multiples, one
// decimal above the byte unit).
func (t *Translator) Rate(bps float64) string {
	keys := []string{"unit.rate.b", "unit.rate.kb", "unit.rate.mb", "unit.rate.gb"}
	i := 0
	for bps >= 1024 && i < len(keys)-1 {
		bps /= 1024
		i++
	}
	if i == 0 {
		return t.Number(math.Round(bps), 0) + " " + t.T(keys[0])
	}
	return t.Number(bps, 1) + " " + t.T(keys[i])
}

// Has reports whether key exists in the language or in the fallback.
func (t *Translator) Has(key string) bool {
	return t.b.msgs[t.code][key] != "" || t.b.msgs[FallbackLang][key] != ""
}

// Memory formats used and total memory in gigabytes ("22,3 / 63,3 Go").
func (t *Translator) Memory(used, total uint64) string {
	const gib = 1 << 30
	return t.T("unit.memory", "used", t.Number(float64(used)/gib, 1), "total", t.Number(float64(total)/gib, 1))
}
