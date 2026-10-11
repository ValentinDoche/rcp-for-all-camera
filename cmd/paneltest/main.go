//go:build windows

// paneltest is a hardware test tool for the ATEM Micro Camera Panel. It
// connects to the panel over USB, shows every button/knob/lever live in a web
// page with a checklist, and drives the panel displays, camera matrix and
// camera-button LEDs with simulated camera data (ND, CC, gain, shutter, iris,
// master black).
//
//	go run ./cmd/paneltest            then open http://localhost:8765
package main

import (
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

//go:embed web
var webFS embed.FS

type control struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

var controls = func() []control {
	var c []control
	for i := 1; i <= 8; i++ {
		c = append(c, control{fmt.Sprintf("CAM%d", i), fmt.Sprintf("Caméra %d", i), "button"})
	}
	for i := 1; i <= 5; i++ {
		c = append(c, control{fmt.Sprintf("SCENE%d", i), fmt.Sprintf("Scene file %d", i), "button"})
	}
	for _, b := range [][2]string{
		{"STORE", "STORE"}, {"ND_UP", "ND ▲"}, {"ND_DOWN", "ND ▼"}, {"CC_UP", "CC ▲"}, {"CC_DOWN", "CC ▼"},
		{"GAIN_UP", "Master gain ▲"}, {"GAIN_DOWN", "Master gain ▼"}, {"SHUTTER_UP", "Shutter ▲"}, {"SHUTTER_DOWN", "Shutter ▼"},
		{"ON", "ON"}, {"WB", "W/B"}, {"BARS", "BARS"}, {"BLACKFLARE", "Bouton Black/Flare"},
		{"IRISMB_ACTIVE", "IRIS/MB ACTIVE"}, {"AUTO_IRIS", "AUTO IRIS"}, {"CALL", "CALL"},
		{"PANEL_ACTIVE", "PANEL ACTIVE"}, {"PREVIEW", "PREVIEW"},
	} {
		c = append(c, control{b[0], b[1], "button"})
	}
	for _, k := range [][2]string{
		{"WHITE_R", "White R"}, {"WHITE_G", "White G"}, {"WHITE_B", "White B"},
		{"BLACK_R", "Black R"}, {"BLACK_G", "Black G"}, {"BLACK_B", "Black B"},
		{"KNOB7", "Molette haut-droite"}, {"SENS", "SENS"}, {"COARSE", "COARSE"},
	} {
		c = append(c, control{k[0], k[1], "knob"})
	}
	return append(c, control{"LEVER", "Levier iris", "lever"}, control{"RING", "Bague master black", "ring"})
}()

type logEntry struct {
	T   string `json:"t"`
	Msg string `json:"msg"`
}

// app holds all state shown in the UI and pushed to the panel.
type app struct {
	mu           sync.Mutex
	panel        *micropanel.Panel
	connected    bool
	serial       string
	firmware     string
	held         map[string]bool
	seen         map[string]bool
	analog       map[string]int
	lever        int32
	ring         int32
	last         micropanel.State
	haveLast     bool
	cams         [9]*fakeCamera // 1..8
	selected     int
	preview      int
	storeArm     bool
	panelOn      bool
	log          []logEntry
	testUntil    time.Time // while set, the render loop leaves the panel alone
	ledExplore   bool      // LED explorer: SHUTTER ▲▼ walks one LED, ND ▲▼ picks the layer
	ledIndex     int
	ledLayer     int
	clearLayers  bool      // other LED layers must be wiped (the explorer used them)
	irisMB       bool      // IRIS/MB ACTIVE: lever and ring drive the camera
	scenes       [9]uint8  // per camera, bit n = scene n+1 stored
	wbFlashUntil time.Time // W/B backlight flashes after a (fake) AWB

	subsMu sync.Mutex
	subs   map[chan []byte]struct{}
	dirty  chan struct{}
}

func newApp() *app {
	a := &app{
		held: map[string]bool{}, seen: map[string]bool{}, analog: map[string]int{},
		selected: 1, preview: 2, panelOn: true, irisMB: true,
		subs: map[chan []byte]struct{}{}, dirty: make(chan struct{}, 1),
	}
	for i := 1; i <= 8; i++ {
		a.cams[i] = newFakeCamera(i)
	}
	return a
}

func (a *app) logf(format string, args ...any) {
	e := logEntry{time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...)}
	a.log = append(a.log, e)
	if len(a.log) > 200 {
		a.log = a.log[len(a.log)-200:]
	}
}

func (a *app) changed() {
	select {
	case a.dirty <- struct{}{}:
	default:
	}
}

// onState diffs a new panel snapshot against the previous one.
func (a *app) onState(s micropanel.State) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer a.changed()

	now := map[string]bool{}
	for _, k := range s.Keys {
		now[micropanel.KeyName(k)] = true
	}
	for id := range now {
		if !a.held[id] {
			a.seen[id] = true
			a.logf("%s ↓", id)
			a.press(id)
		}
	}
	for id := range a.held {
		if !now[id] {
			a.logf("%s ↑", id)
		}
	}
	a.held = now

	cam := a.cams[a.selected]
	for i, v := range s.Analog {
		id := micropanel.AnalogNames[i]
		if a.haveLast && v != a.last.Analog[i] {
			a.seen[id] = true
			if old := a.analog[id]; absInt(int(v)-old) >= 64 || v == 0 || v == micropanel.AnalogMax {
				a.logf("%s = %d", id, v)
			}
			norm := float64(v)/micropanel.AnalogMax*2 - 1
			switch id {
			case "WHITE_R", "WHITE_G", "WHITE_B":
				cam.White[strings.Index("RGB", id[6:])] = norm
			case "BLACK_R", "BLACK_G", "BLACK_B":
				cam.Black[strings.Index("RGB", id[6:])] = norm
			}
		}
		if !a.haveLast || absInt(int(v)-a.analog[id]) >= 64 || v == 0 || v == micropanel.AnalogMax {
			a.analog[id] = int(v)
		}
	}
	if a.haveLast && s.Lever != a.last.Lever {
		a.seen["LEVER"] = true
		if !cam.AutoIris && a.irisMB {
			cam.setIrisFromLever(s.Lever)
		}
		if absInt(int(s.Lever-a.lever)) >= 512 {
			a.logf("LEVER = %d (%s)", s.Lever, cam.irisText())
			a.lever = s.Lever
		}
	}
	if a.haveLast && s.Ring != a.last.Ring {
		a.seen["RING"] = true
		if a.irisMB {
			cam.MasterBlack = clamp(int(s.Ring)*99/micropanel.JoystickMax, -99, 99)
		}
		if absInt(int(s.Ring-a.ring)) >= 512 {
			a.logf("RING = %d (MB %d)", s.Ring, cam.MasterBlack)
			a.ring = s.Ring
		}
	}
	if !a.haveLast {
		a.lever, a.ring = s.Lever, s.Ring
		for _, c := range a.cams[1:] {
			c.setIrisFromLever(s.Lever)
			c.MasterBlack = clamp(int(s.Ring)*99/micropanel.JoystickMax, -99, 99)
		}
	}
	a.last, a.haveLast = s, true
}

// press applies the simulated effect of a button.
func (a *app) press(id string) {
	if a.ledExplore {
		switch id {
		case "SHUTTER_UP":
			a.ledIndex = (a.ledIndex + 1) % 240
		case "SHUTTER_DOWN":
			a.ledIndex = (a.ledIndex + 239) % 240
		case "ND_UP":
			a.ledLayer = (a.ledLayer + 1) % 6
			a.clearLayers = true
		case "ND_DOWN":
			a.ledLayer = (a.ledLayer + 5) % 6
			a.clearLayers = true
		default:
			return
		}
		a.logf("Explorateur LED: LED %d, couche %d", a.ledIndex, a.ledLayer)
		return
	}
	switch {
	case strings.HasPrefix(id, "CAM"):
		n := int(id[3] - '0')
		if a.held["PREVIEW"] {
			a.preview = n
		} else {
			a.selected = n
		}
	case strings.HasPrefix(id, "SCENE"):
		n := id[5:]
		if a.storeArm {
			a.logf("Scene %s mémorisée (simulé)", n)
			a.scenes[a.selected] |= 1 << (n[0] - '1')
			a.storeArm = false
		} else {
			a.logf("Scene %s rappelée (simulé)", n)
		}
	case id == "STORE":
		a.storeArm = !a.storeArm
	case id == "PANEL_ACTIVE":
		a.panelOn = !a.panelOn
	case id == "IRISMB_ACTIVE":
		a.irisMB = !a.irisMB
	case id == "WB":
		a.cams[a.selected].step(id)
		a.wbFlashUntil = time.Now().Add(time.Second)
		time.AfterFunc(1100*time.Millisecond, a.changed)
	default:
		a.cams[a.selected].step(id)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// snapshot builds the JSON state for the UI.
func (a *app) snapshot() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	cam := a.cams[a.selected]
	held := []string{}
	for id := range a.held {
		held = append(held, id)
	}
	leds := map[string]any{"NETWORK": a.connected, "ALARM": cam.GainDb >= 30, "CABLE": a.connected,
		"REL": cam.On, "ABS": !cam.On, "BARS_IND": cam.Bars, "EXT": cam.GainDb >= 12, "DEXT": cam.GainDb >= 24}
	for i := 1; i <= 8; i++ {
		c := "off"
		switch i {
		case a.selected:
			c = "red"
		case a.preview:
			c = "green"
		}
		leds[fmt.Sprintf("CAM%d", i)] = c
	}
	logCopy := make([]logEntry, len(a.log))
	for i, e := range a.log {
		logCopy[len(a.log)-1-i] = e
	}
	iris := cam.irisText()
	if cam.AutoIris {
		iris = "Auto"
	}
	st := map[string]any{
		"connected": a.connected, "authenticated": a.connected, "serial": a.serial, "firmware": a.firmware,
		"held": held, "seen": a.seen, "controls": controls, "analog": a.analog,
		"lever": a.last.Lever, "ring": a.last.Ring,
		"displays": map[string]string{
			"ND": cam.ndDigit(), "CC": ccSteps[cam.CC], "GAIN": cam.gainText(), "SHUTTER": cam.shutterDigits(),
			"IRIS": iris, "MB": cam.mbText(), "CAMERA": fmt.Sprint(a.selected),
		},
		"leds": leds,
		"cam": map[string]any{
			"selected": a.selected, "preview": a.preview, "nd": ndSteps[cam.ND], "cc": ccSteps[cam.CC],
			"gainDb": cam.GainDb, "shutter": cam.shutterText(), "iris": cam.irisText(), "masterBlack": cam.MasterBlack,
			"wbK": cam.WbK, "white": cam.White, "black": cam.Black, "autoIris": cam.AutoIris, "bars": cam.Bars,
			"panelActive": a.panelOn, "storeArmed": a.storeArm, "irisMBActive": a.irisMB,
		},
		"ledExplorer": map[string]any{"on": a.ledExplore, "index": a.ledIndex, "layer": a.ledLayer},
		"log":         logCopy,
	}
	b, _ := json.Marshal(st)
	return b
}

// renderPanel pushes the simulated camera data to the panel displays and LEDs.
func (a *app) renderPanel() {
	a.mu.Lock()
	p := a.panel
	if p == nil || time.Now().Before(a.testUntil) {
		a.mu.Unlock()
		return
	}
	var seg [14]byte
	var leds micropanel.LEDBits
	var matrix [32]byte
	layer := byte(0)
	if a.ledExplore {
		micropanel.EncodeText(&seg, micropanel.DispND, fmt.Sprint(a.ledLayer))
		micropanel.EncodeText(&seg, micropanel.DispShutter, fmt.Sprint(a.ledIndex))
		micropanel.EncodeText(&seg, micropanel.DispIris, "LEd")
		leds.Set(a.ledIndex, true)
		layer = byte(a.ledLayer)
	} else {
		cam := a.cams[a.selected]
		micropanel.EncodeText(&seg, micropanel.DispND, cam.ndDigit())
		micropanel.EncodeText(&seg, micropanel.DispCC, ccSteps[cam.CC])
		micropanel.EncodeText(&seg, micropanel.DispGain, cam.gainText())
		micropanel.EncodeText(&seg, micropanel.DispShutter, cam.shutterDigits())
		if cam.AutoIris {
			micropanel.EncodeText(&seg, micropanel.DispIris, "Aut")
		} else {
			micropanel.EncodeText(&seg, micropanel.DispIris, cam.irisText())
		}
		micropanel.EncodeText(&seg, micropanel.DispMasterBlack, cam.mbText())
		for i := 1; i <= 8; i++ {
			switch i {
			case a.selected:
				leds.SetCamera(i, micropanel.Red)
			case a.preview:
				leds.SetCamera(i, micropanel.Green)
			}
		}
		a.setIndicators(&leds, cam)
		colour := byte(micropanel.PixRed)
		if !a.panelOn {
			colour = micropanel.PixAmber
		}
		matrix = micropanel.MatrixNumber(a.selected, colour)
	}
	clearLayers := a.clearLayers
	a.clearLayers = false
	a.mu.Unlock()

	if clearLayers {
		if err := p.ClearLEDs(int(layer)); err != nil {
			log.Printf("clear LEDs: %v", err)
		}
	}
	for _, err := range []error{p.SetDigitsRaw(seg), p.SetMatrixRaw(matrix), p.WriteReport(leds.Report(layer))} {
		if err != nil {
			log.Printf("panel write: %v", err)
		}
	}
}

// setIndicators drives the button backlights and indicator LEDs from the
// simulated state. Caller holds a.mu.
func (a *app) setIndicators(l *micropanel.LEDBits, cam *fakeCamera) {
	// Arrows are lit when the value can still move in that direction.
	l.Set(micropanel.LEDNDUp, cam.ND < len(ndSteps)-1)
	l.Set(micropanel.LEDNDDown, cam.ND > 0)
	l.Set(micropanel.LEDCCUp, cam.CC < len(ccSteps)-1)
	l.Set(micropanel.LEDCCDown, cam.CC > 0)
	l.Set(micropanel.LEDGainUp, cam.GainDb < 36)
	l.Set(micropanel.LEDGainDown, cam.GainDb > -6)
	l.Set(micropanel.LEDShutterUp, cam.Shutter < len(shutterSteps)-1)
	l.Set(micropanel.LEDShutterDown, cam.Shutter > 0)

	for i := 0; i < 5; i++ {
		l.Set(micropanel.LEDScene1+i, a.scenes[a.selected]&(1<<i) != 0 || a.held[fmt.Sprintf("SCENE%d", i+1)])
	}
	l.Set(micropanel.LEDSceneFile, a.storeArm)
	l.Set(micropanel.LEDStoreRed, a.storeArm)
	l.Set(micropanel.LEDStoreWhite, !a.storeArm)

	l.Set(micropanel.LEDOn, cam.On)
	l.Set(micropanel.LEDRel, cam.On)
	l.Set(micropanel.LEDAbs, !cam.On)
	l.Set(micropanel.LEDWB, time.Now().Before(a.wbFlashUntil))
	l.Set(micropanel.LEDBars, cam.Bars)
	l.Set(micropanel.LEDBarsInd, cam.Bars)
	l.Set(micropanel.LEDExt, cam.GainDb >= 12)
	l.Set(micropanel.LEDDExt, cam.GainDb >= 24)

	l.Set(micropanel.LEDIrisMBRed, a.irisMB)
	l.Set(micropanel.LEDIrisMBWhite, !a.irisMB)
	l.Set(micropanel.LEDAutoIris, cam.AutoIris)
	l.Set(micropanel.LEDCallRed, a.held["CALL"])
	l.Set(micropanel.LEDCallWhite, !a.held["CALL"])
	l.Set(micropanel.LEDPanelRed, a.panelOn)
	l.Set(micropanel.LEDPanelWhite, !a.panelOn)
	l.Set(micropanel.LEDPreview, a.held["PREVIEW"])

	l.Set(micropanel.LEDNetwork, a.connected)
	l.Set(micropanel.LEDCable, a.connected)
	l.Set(micropanel.LEDAlarm, cam.GainDb >= 30)

	// Iris scale: labels 2.0, 2.8, 4 … 16 sit on odd segments, two segments
	// per stop; the bar runs from the current opening down to CLS.
	stops := 2 * math.Log2(cam.Iris/2.0)
	l.SetIrisBar(clamp(int(math.Round(1+2*stops)), 0, micropanel.LEDIrisBarCount-1))
}

func (a *app) broadcastLoop() {
	tick := time.NewTicker(33 * time.Millisecond)
	defer tick.Stop()
	for range tick.C {
		select {
		case <-a.dirty:
		default:
			continue
		}
		a.renderPanel()
		b := a.snapshot()
		a.subsMu.Lock()
		for c := range a.subs {
			select {
			case c <- b:
			default:
			}
		}
		a.subsMu.Unlock()
	}
}

// panelLoop connects to the panel and reconnects after unplug/errors.
func (a *app) panelLoop() {
	for {
		p, err := micropanel.Open()
		if err != nil {
			a.mu.Lock()
			wasConnected := a.connected
			a.connected = false
			a.mu.Unlock()
			if wasConnected {
				log.Printf("panel: %v", err)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		a.mu.Lock()
		a.panel, a.connected, a.serial, a.firmware = p, true, p.Serial(), p.Firmware
		a.haveLast = false
		a.logf("Panneau connecté (serial %s, session %s)", p.Serial(), p.AuthTTL())
		a.mu.Unlock()
		a.changed()
		log.Printf("panel connected, serial %s", p.Serial())

		err = p.Run(context.Background(), a.onState, nil)

		a.mu.Lock()
		a.panel, a.connected = nil, false
		a.logf("Panneau déconnecté: %v", err)
		a.mu.Unlock()
		a.changed()
		log.Printf("panel lost: %v", err)
		time.Sleep(time.Second)
	}
}

// runTest plays a test pattern on the panel, or toggles the LED explorer.
func (a *app) runTest(kind string) error {
	if kind == "leds" {
		a.mu.Lock()
		a.ledExplore = !a.ledExplore
		a.clearLayers = true
		state := "désactivé"
		if a.ledExplore {
			state = "activé (SHUTTER ▲▼ = LED suivante/précédente, ND ▲▼ = couche)"
		}
		a.logf("Explorateur LED %s", state)
		a.mu.Unlock()
		a.changed()
		return nil
	}
	a.mu.Lock()
	p := a.panel
	a.testUntil = time.Now().Add(10 * time.Second)
	a.logf("Test %s", kind)
	a.mu.Unlock()
	if p == nil {
		return fmt.Errorf("panneau non connecté")
	}
	go func() {
		switch kind {
		case "displays":
			for _, s := range []string{"8.", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "A", "b", "C", "d", "E", "F", "-"} {
				var seg [14]byte
				for d := micropanel.DispND; d <= micropanel.DispMasterBlack; d++ {
					micropanel.EncodeText(&seg, d, strings.Repeat(s, micropanel.DisplayWidth(d)))
				}
				p.SetDigitsRaw(seg)
				time.Sleep(300 * time.Millisecond)
			}
		case "matrix":
			for _, c := range []byte{micropanel.PixRed, micropanel.PixGreen, micropanel.PixAmber} {
				for n := 1; n <= 12; n++ {
					p.SetMatrixRaw(micropanel.MatrixNumber(n, c))
					time.Sleep(150 * time.Millisecond)
				}
			}
			var leds micropanel.LEDBits
			for _, c := range []micropanel.Colour{micropanel.Red, micropanel.Green, micropanel.Blue, micropanel.Amber, micropanel.Cyan, micropanel.Magenta, micropanel.White} {
				for i := 1; i <= 8; i++ {
					leds.SetCamera(i, c)
				}
				p.WriteReport(leds.Report(0))
				time.Sleep(250 * time.Millisecond)
			}
		}
		a.mu.Lock()
		a.testUntil = time.Time{}
		a.mu.Unlock()
		a.changed()
	}()
	return nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "HTTP listen address")
	noBrowser := flag.Bool("no-browser", false, "don't open the browser")
	flag.Parse()

	a := newApp()
	go a.panelLoop()
	go a.broadcastLoop()

	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(sub))
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		c := make(chan []byte, 4)
		a.subsMu.Lock()
		a.subs[c] = struct{}{}
		a.subsMu.Unlock()
		defer func() {
			a.subsMu.Lock()
			delete(a.subs, c)
			a.subsMu.Unlock()
		}()
		fl, _ := w.(http.Flusher)
		send := func(b []byte) bool {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return false
			}
			if fl != nil {
				fl.Flush()
			}
			return true
		}
		if !send(a.snapshot()) {
			return
		}
		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case b := <-c:
				if !send(b) {
					return
				}
			case <-keepalive.C:
				fmt.Fprint(w, ": ping\n\n")
				if fl != nil {
					fl.Flush()
				}
			}
		}
	})
	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.seen = map[string]bool{}
		a.logf("Checklist réinitialisée")
		a.mu.Unlock()
		a.changed()
	})
	mux.HandleFunc("POST /api/test/{kind}", func(w http.ResponseWriter, r *http.Request) {
		if err := a.runTest(r.PathValue("kind")); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		}
	})
	// Raw access for protocol exploration: POST /api/raw?out=10:<hex>. The
	// render loop pauses for an hour; POST /api/resume to give it back.
	mux.HandleFunc("POST /api/raw", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		p := a.panel
		a.testUntil = time.Now().Add(time.Hour)
		a.mu.Unlock()
		if p == nil {
			http.Error(w, "panneau non connecté", http.StatusServiceUnavailable)
			return
		}
		id, payload, _ := strings.Cut(r.URL.Query().Get("out"), ":")
		b, err := hex.DecodeString(id + payload)
		if err != nil || len(b) == 0 {
			http.Error(w, "bad hex", http.StatusBadRequest)
			return
		}
		if err := p.WriteReport(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /api/resume", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.testUntil = time.Time{}
		a.mu.Unlock()
		a.changed()
	})

	url := "http://" + *addr
	log.Printf("paneltest on %s", url)
	if !*noBrowser {
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}
