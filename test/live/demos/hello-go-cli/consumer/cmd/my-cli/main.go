// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: CC0-1.0

// Command my-cli greets the name it is given.
package main

import (
	"fmt"
	"os"
)

// The release build sets these with -ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	if name == "--version" {
		fmt.Printf("my-cli %s (%s)\n", version, commit)

		return
	}

	fmt.Println(greeting(name))
}

func greeting(name string) string {
	if name == "" {
		name = "world"
	}

	return "Hello, " + name + "!"
}
