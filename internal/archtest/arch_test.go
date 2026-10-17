// Package archtest checks the dependency rules of the architecture
// (docs/go-engineering-rules.md, rules A1 and X1): the core depends on
// nothing, the application on no adapter, a module on nothing but the SDK.
package archtest

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

const repo = "github.com/valentindoche/rcp-for-all-camera/"

// imports lists the direct imports of every package of the repository.
func imports(t *testing.T) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", repo+"...")
	cmd.Dir = "../.."
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	m := map[string][]string{}
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) > 0 {
			m[strings.TrimPrefix(f[0], repo)] = f[1:]
		}
	}
	return m
}

// rule forbids packages whose path starts with from to import a path
// starting with any of forbidden (repository paths without the module).
type rule struct {
	from      string
	forbidden []string
	why       string
}

var rules = []rule{
	{"internal/core", []string{repo, "net/http", "os", "fyne.io/"}, "the core is the contract: no adapter, no I/O, no framework"},
	{"internal/app", []string{repo + "internal/config", repo + "internal/gui", repo + "internal/plugins", repo + "internal/surfaces", repo + "internal/sysinfo", "fyne.io/", "net/http"}, "the application uses ports; adapters are injected by cmd/rcp"},
	{"internal/plugins", []string{repo + "internal/app", repo + "internal/gui", repo + "internal/config", repo + "internal/surfaces", "fyne.io/"}, "the plugin host only turns modules into core cameras"},
	{"internal/gui", []string{repo + "internal/config", repo + "internal/plugins", repo + "internal/sysinfo", repo + "internal/surfaces/micropanel"}, "the GUI talks to the application, not to other adapters"},
	{"sdk/", []string{repo + "internal/", repo + "modules/", "fyne.io/"}, "the SDK is public: no internal package"},
	{"modules/", []string{repo + "internal/", repo + "modules/", "fyne.io/", "net/http", "os/exec"}, "a module is independent and sandboxed: only the SDK"},
}

func TestDependencyRules(t *testing.T) {
	for pkg, deps := range imports(t) {
		for _, r := range rules {
			if !strings.HasPrefix(pkg, r.from) {
				continue
			}
			for _, d := range deps {
				for _, f := range r.forbidden {
					if strings.HasPrefix(d, f) && !allowed(pkg, d) {
						t.Errorf("%s imports %s: %s", pkg, d, r.why)
					}
				}
			}
		}
	}
}

// allowed is the one exception: a package may import its own
// sub-packages (a module could split itself into sub-packages; the SDK
// test helpers live under the SDK).
func allowed(pkg, dep string) bool {
	return strings.HasPrefix(dep, repo+pkg+"/")
}
