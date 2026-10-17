package plugins

import (
	"encoding/json"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// FakeBlackmagic serves the Blackmagic REST API and event WebSocket from
// an in-memory state, for end-to-end tests of the bm-studio-6k-pro module
// over a real local network. A PUT merges into the property and is pushed
// on every socket. Exported for the plugins_test package only.
type FakeBlackmagic struct {
	srv   *httptest.Server
	mu    sync.Mutex
	state map[string]map[string]any
	puts  []string
	socks []*wsConn
}

const fakeAPI = "/control/api/v1"

func NewFakeBlackmagic(t *testing.T) *FakeBlackmagic {
	f := &FakeBlackmagic{state: map[string]map[string]any{
		"/system/format":                    {"frameRate": "50", "offSpeedEnabled": false},
		"/lens/iris":                        {"apertureStop": 4.0, "normalised": 0.25},
		"/lens/focus":                       {"normalised": 0.0},
		"/lens/zoom":                        {"focalLength": 24.0, "normalised": 0.0},
		"/video/gain":                       {"gain": 0.0},
		"/video/iso":                        {"iso": 400.0},
		"/video/shutter":                    {"shutterSpeed": 100.0},
		"/video/ndFilter":                   {"stop": 0.0},
		"/video/whiteBalance":               {"whiteBalance": 4400.0},
		"/video/whiteBalanceTint":           {"whiteBalanceTint": 10.0},
		"/transports/0/record":              {"recording": false},
		"/colorCorrection/lift":             {"red": 0.0, "green": 0.0, "blue": 0.0, "luma": 0.0},
		"/colorCorrection/gamma":            {"red": 0.0, "green": 0.0, "blue": 0.0, "luma": 0.0},
		"/colorCorrection/gain":             {"red": 1.0, "green": 1.0, "blue": 1.0, "luma": 1.0},
		"/colorCorrection/offset":           {"red": 0.0, "green": 0.0, "blue": 0.0, "luma": 0.0},
		"/colorCorrection/contrast":         {"pivot": 0.5, "adjust": 1.0},
		"/colorCorrection/color":            {"hue": 0.0, "saturation": 1.0},
		"/colorCorrection/lumaContribution": {"lumaContribution": 1.0},
		"/lens/iris/description":            {"apertureStop": map[string]any{"min": 2.8, "max": 22.0}},
		"/lens/zoom/description":            {"focalLength": map[string]any{"min": 24.0, "max": 70.0}},
		"/lens/focus/description":           {"focusDistance": map[string]any{"min": 380.0, "max": 1e7}},
	}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// HostPort is where the fake listens.
func (f *FakeBlackmagic) HostPort() (string, int) {
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(f.srv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	return host, p
}

// Puts lists the paths written so far.
func (f *FakeBlackmagic) Puts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.puts...)
}

// DropSockets cuts every open socket, as a cable pull would.
func (f *FakeBlackmagic) DropSockets() {
	f.mu.Lock()
	socks := f.socks
	f.socks = nil
	f.mu.Unlock()
	for _, s := range socks {
		_ = s.Close()
	}
}

func (f *FakeBlackmagic) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == fakeAPI+"/event/websocket" {
		f.serveSocket(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, fakeAPI)
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.state[path]
	switch r.Method {
	case http.MethodGet:
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, _ := json.Marshal(o)
		_, _ = w.Write(b)
	case http.MethodPut:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.puts = append(f.puts, path)
		if ok {
			o = maps.Clone(o)
			maps.Copy(o, body)
			f.state[path] = o
			f.pushLocked(path, o)
		}
	}
}

func (f *FakeBlackmagic) serveSocket(w http.ResponseWriter, r *http.Request) {
	conn, rw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + wsAccept(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n")
	_ = rw.Flush()
	ws := &wsConn{c: conn, br: rw.Reader}
	f.mu.Lock()
	f.socks = append(f.socks, ws)
	f.mu.Unlock()
	for {
		msg, err := ws.readText()
		if err != nil {
			return
		}
		var req struct {
			ID   int `json:"id"`
			Data struct {
				Properties []string `json:"properties"`
			} `json:"data"`
		}
		_ = json.Unmarshal(msg, &req)
		vals, ok := map[string]any{}, true
		f.mu.Lock()
		for _, p := range req.Data.Properties {
			if o, known := f.state[p]; known {
				vals[p] = o
			} else {
				ok = false
			}
		}
		f.mu.Unlock()
		if !ok {
			vals = map[string]any{}
		}
		b, _ := json.Marshal(map[string]any{"type": "response", "id": req.ID, "data": map[string]any{
			"action": "subscribe", "properties": req.Data.Properties, "success": ok, "values": vals,
		}})
		_ = ws.writeText(b)
	}
}

func (f *FakeBlackmagic) pushLocked(path string, o map[string]any) {
	b, _ := json.Marshal(map[string]any{"type": "event", "data": map[string]any{
		"action": "propertyValueChanged", "property": path, "value": o,
	}})
	for _, s := range f.socks {
		_ = s.writeText(b)
	}
}
