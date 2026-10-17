// Command buildmodules compiles the camera modules: every folder of
// modules/ with a main.go becomes <folder>.wasm in the output folder.
//
//	go run ./cmd/buildmodules                    # all modules into dist/modules
//	go run ./cmd/buildmodules -o out demo        # only the demo module
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/valentindoche/rcp-for-all-camera/internal/modbuild"
)

func main() {
	src := flag.String("src", "modules", "folder of the module sources")
	out := flag.String("o", "dist/modules", "output folder")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := modbuild.Build(ctx, *src, *out, flag.Args()...); err != nil {
		fmt.Fprintln(os.Stderr, "buildmodules:", err)
		os.Exit(1)
	}
}
