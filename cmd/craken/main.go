package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("craken %s\n", version)
		return
	}

	if len(os.Args) <= 1 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printUsage()
		if len(os.Args) <= 1 {
			os.Exit(2)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", os.Args[1])
	printUsage()
	os.Exit(2)
}

func printUsage() {
	fmt.Print(`Usage: craken <command> [options]

Commands:
  version     Print the craken CLI version
  help        Show this help message

https://github.com/corca-ai/craken-cli
`)
}
