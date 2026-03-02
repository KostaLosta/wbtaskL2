package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	cfg := parseFlags()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "grep: missing pattern")
		os.Exit(2)
	}

	cfg.pattern = args[0]
	if len(args) > 1 {
		cfg.files = args[1:]
	}

	if cfg.context > 0 {
		cfg.before = cfg.context
		cfg.after = cfg.context
	}

	matchFunc, err := buildMatcher(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep: invalid pattern: %v\n", err)
		os.Exit(2)
	}

	if len(cfg.files) == 0 {
		if err := processStream("", os.Stdin, &cfg, matchFunc, false); err != nil {
			fmt.Fprintf(os.Stderr, "grep: %v\n", err)
			os.Exit(1)
		}
		return
	}

	multipleFiles := len(cfg.files) > 1
	exitCode := 0

	for _, name := range cfg.files {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "grep: %s: %v\n", name, err)
			exitCode = 1
			continue
		}
		if err := processStream(name, f, &cfg, matchFunc, multipleFiles); err != nil {
			// Ошибки чтения файла — как в обычном grep — сообщаем, но идём дальше.
			fmt.Fprintf(os.Stderr, "grep: %s: %v\n", name, err)
			exitCode = 1
		}
		_ = f.Close()
	}

	os.Exit(exitCode)
}
