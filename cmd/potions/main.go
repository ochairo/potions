package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()
	command := os.Args[1]

	// Dispatch to subcommand
	switch command {
	case "build":
		runBuild(ctx, os.Args[2:])
	case "list":
		runList(ctx, os.Args[2:])
	case "scan":
		runScan(ctx, os.Args[2:])
	case "verify":
		runVerify(ctx, os.Args[2:])
	case "monitor":
		runMonitor(ctx, os.Args[2:])
	case "release":
		runRelease(ctx, os.Args[2:])
	case "validate-release":
		runValidateRelease(ctx, os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`potions - Automated binary builder and release manager

Usage:
  potions <command> [options]

Commands:
  build             Build binaries for one or more packages
  list              List available package recipes
  scan              Run security scan on a package/binary
  verify            Verify checksums and signatures
  monitor           Check for version updates
  release           Create single or batch GitHub releases
  validate-release  Validate platform coverage for release

Use "potions <command> --help" for more information about a command.`)
}

func detectPlatform() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go's GOARCH to common platform names
	archMap := map[string]string{
		"amd64": "x86_64",
		"arm64": "arm64",
		"386":   "i386",
	}

	mappedArch := archMap[arch]
	if mappedArch == "" {
		mappedArch = arch
	}

	return fmt.Sprintf("%s-%s", os, mappedArch)
}
