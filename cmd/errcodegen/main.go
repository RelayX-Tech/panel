package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/relayx-tech/panel/internal/errcode"
)

func main() {
	in := flag.String("in", "api/error-codes.yaml", "error code registry YAML")
	goOut := flag.String("go", "internal/errcode/codes_gen.go", "generated Go file")
	tsOut := flag.String("ts", "", "generated TypeScript file (optional)")
	msgDir := flag.String("messages", "", "generated messages directory (optional)")
	flag.Parse()

	codes, err := errcode.LoadFile(*in)
	if err != nil {
		fail(err)
	}

	if *goOut != "" {
		data, err := errcode.GenerateGo(codes)
		if err != nil {
			fail(err)
		}
		write(*goOut, data)
	}
	if *tsOut != "" {
		data, err := errcode.GenerateTS(codes)
		if err != nil {
			fail(err)
		}
		write(*tsOut, data)
	}
	if *msgDir != "" {
		for _, locale := range []string{"zh-CN", "en-US"} {
			data, err := errcode.GenerateMessages(codes, locale)
			if err != nil {
				fail(err)
			}
			write(filepath.Join(*msgDir, "errors."+locale+".json"), data)
		}
	}
}

func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s\n", path)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "errcodegen:", err)
	os.Exit(1)
}
