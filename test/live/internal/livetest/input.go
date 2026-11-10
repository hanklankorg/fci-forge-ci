// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"os"
	"strings"
	"sync"
)

// inputs holds, by test name, what SetInputs gave a test.
var inputs sync.Map

// SetInputs gives t and its subtests their own inputs: what a table case
// would otherwise have to put in the process environment, where two cases
// running at once would read each other's. They last as long as t does.
func SetInputs(t interface {
	Name() string
	Cleanup(f func())
}, values map[string]string,
) {
	name := t.Name()

	inputs.Store(name, values)
	t.Cleanup(func() { inputs.Delete(name) })
}

// Input is the value of the input name for tb: the one SetInputs gave tb or
// its nearest parent, and otherwise the process environment's, which is where
// `just test-live-consumer` and the launcher put a run's inputs. A test the
// suite did not name reads the environment alone.
func Input(tb TB, name string) string {
	named, ok := tb.(interface{ Name() string })
	if !ok {
		return os.Getenv(name)
	}

	for test := named.Name(); test != ""; {
		if values, found := inputs.Load(test); found {
			if value, set := values.(map[string]string)[name]; set {
				return value
			}
		}

		parent := strings.LastIndexByte(test, '/')
		if parent < 0 {
			break
		}

		test = test[:parent]
	}

	return os.Getenv(name)
}
