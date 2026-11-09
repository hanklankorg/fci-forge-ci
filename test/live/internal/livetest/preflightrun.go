// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	// livePreflightEnv makes a live run a preflight: each scenario runs its
	// admission, the checks it makes before any effect, and stops there.
	livePreflightEnv = "FORGE_CI_LIVE_PREFLIGHT"
	// liveAdmittedPrefix marks, in the run's log, a scenario whose admission
	// a preflight run completed.
	liveAdmittedPrefix = "FORGE_CI_LIVE_ADMITTED "
	// verdictAdmitted is a preflight run in which every scenario that ran
	// was admitted.
	verdictAdmitted = "admitted"
)

// errPreflightEffect refuses a lab client in a preflight run: an admission
// reads the contract, the host and anonymous registries, never the lab with
// its credential.
var errPreflightEffect = errors.New("a preflight run reached a lab effect; end the scenario's admission with Admitted before its first effect")

// errUnadmittedScenario refuses, in a preflight run, a test that reaches the
// lab contract without an admission boundary.
var errUnadmittedScenario = errors.New("reached the lab contract in a preflight run without RegisterAdmittedScenario")

// admittedScenarios is every scenario of this process that registered with
// RegisterAdmittedScenario, by test name.
var admittedScenarios sync.Map

// PreflightRun reports whether this live run is a preflight.
func PreflightRun() bool {
	return os.Getenv(livePreflightEnv) == "1"
}

// RegisterAdmittedScenario is RegisterLiveScenario for a scenario that ends
// its admission with Admitted. Only such a scenario runs in a preflight run;
// it runs there until Admitted.
func RegisterAdmittedScenario(t interface{ Name() string }) {
	admittedScenarios.Store(t.Name(), true)
	fmt.Printf("%s%s\n", liveScenarioPrefix, t.Name())
}

// requireAdmittedScenario refuses, in a preflight run, a test that reaches
// the lab contract without having registered, itself or a parent, with
// RegisterAdmittedScenario: nothing would stop it before its effects.
func requireAdmittedScenario(tb TB) error {
	if !PreflightRun() {
		return nil
	}

	named, ok := tb.(interface{ Name() string })
	if !ok {
		return fmt.Errorf("an unnamed test %w", errUnadmittedScenario)
	}

	for name := named.Name(); ; {
		if _, registered := admittedScenarios.Load(name); registered {
			return nil
		}

		parent := strings.LastIndex(name, "/")
		if parent < 0 {
			return fmt.Errorf("%s %w", named.Name(), errUnadmittedScenario)
		}

		name = name[:parent]
	}
}

// Admitted ends a scenario's admission: every check it makes before any
// effect is behind it. A preflight run reports the scenario admitted and
// stops it here; any other run goes on.
func Admitted(tb TB) {
	tb.Helper()

	if !PreflightRun() {
		return
	}

	if named, ok := tb.(interface{ Name() string }); ok {
		fmt.Printf("%s%s\n", liveAdmittedPrefix, named.Name())
	}

	tb.Skipf("preflight: admitted; the run would go on to its first effect here")
}

// skipUnadmittedInPreflight stops, in a preflight run, a scenario that
// declares no admission boundary: nothing says where its effects begin, so
// it does not run at all.
func skipUnadmittedInPreflight(t any) {
	if !PreflightRun() {
		return
	}

	skipper, ok := t.(interface {
		Skipf(format string, args ...any)
	})
	if !ok {
		panic("livetest: a preflight run cannot stop this scenario")
	}

	skipper.Skipf("preflight: this scenario declares no admission boundary (RegisterAdmittedScenario and Admitted), so a preflight cannot run it without effects")
}
