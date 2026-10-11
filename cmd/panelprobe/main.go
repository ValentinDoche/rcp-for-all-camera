//go:build windows

// panelprobe is a low-level tool for reverse-engineering the ATEM Micro Camera
// Panel: it authenticates, optionally writes raw reports, then dumps every
// input report as hex.
//
//	panelprobe -t 10s
//	panelprobe -out 10:3f065b4f666d7d077f6f777c395e -t 0
//	panelprobe -feat 12 -t 0
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	var outs, feats, gets multi
	flag.Var(&outs, "out", "write output report: <id hex>:<payload hex> (repeatable)")
	flag.Var(&feats, "feat", "write feature report: <id hex>:<payload hex> (repeatable)")
	flag.Var(&gets, "get", "read feature report: <id hex>:<total size> (repeatable)")
	demo := flag.Bool("demo", false, "show sample values on displays, matrix and camera LEDs")
	dur := flag.Duration("t", 10*time.Second, "how long to dump input reports (0 = don't)")
	flag.Parse()

	p, err := micropanel.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()
	fmt.Printf("panel serial=%s firmware=%s auth=%s\n", p.Serial(), p.Firmware, p.AuthTTL())

	if *demo {
		var seg [14]byte
		micropanel.EncodeText(&seg, micropanel.DispND, "2")
		micropanel.EncodeText(&seg, micropanel.DispCC, "A")
		micropanel.EncodeText(&seg, micropanel.DispGain, "12")
		micropanel.EncodeText(&seg, micropanel.DispShutter, "50")
		micropanel.EncodeText(&seg, micropanel.DispIris, "F2.8")
		micropanel.EncodeText(&seg, micropanel.DispMasterBlack, "-05")
		var leds micropanel.LEDBits
		leds.SetCamera(1, micropanel.Red)
		leds.SetCamera(2, micropanel.Green)
		leds.SetCamera(3, micropanel.Cyan)
		leds.SetCamera(4, micropanel.Amber)
		for _, err := range []error{p.SetDigitsRaw(seg), p.SetMatrixRaw(micropanel.MatrixDigit(1)), p.WriteReport(leds.Report(0))} {
			if err != nil {
				log.Print(err)
			}
		}
	}
	for _, f := range feats {
		b := mustReport(f)
		if err := p.SetFeature(b); err != nil {
			log.Printf("feat %s: %v", f, err)
		}
	}
	for _, o := range outs {
		b := mustReport(o)
		if err := p.WriteReport(b); err != nil {
			log.Printf("out %s: %v", o, err)
		}
	}
	for _, g := range gets {
		id, size, _ := strings.Cut(g, ":")
		i, _ := strconv.ParseUint(id, 16, 8)
		n, _ := strconv.Atoi(size)
		r, err := p.GetFeature(byte(i), n)
		if err != nil {
			log.Printf("get %s: %v", g, err)
			continue
		}
		fmt.Printf("feature %s\n", hex.EncodeToString(r))
	}
	if *dur == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()
	start := time.Now()
	err = p.Run(ctx, func(micropanel.State) {}, func(r []byte) {
		fmt.Printf("%8.3f %s\n", time.Since(start).Seconds(), hex.EncodeToString(r))
	})
	if err != nil && ctx.Err() == nil {
		log.Print(err)
		os.Exit(1)
	}
}

func mustReport(s string) []byte {
	id, payload, _ := strings.Cut(s, ":")
	i, err := strconv.ParseUint(id, 16, 8)
	if err != nil {
		log.Fatalf("bad report id in %q", s)
	}
	b, err := hex.DecodeString(payload)
	if err != nil {
		log.Fatalf("bad hex in %q", s)
	}
	return append([]byte{byte(i)}, b...)
}
