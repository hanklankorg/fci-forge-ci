// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: CC0-1.0

package main

import "testing"

func TestGreeting(t *testing.T) {
	for name, want := range map[string]string{"": "Hello, world!", "Ada": "Hello, Ada!"} {
		if got := greeting(name); got != want {
			t.Errorf("greeting(%q) = %q, want %q", name, got, want)
		}
	}
}
