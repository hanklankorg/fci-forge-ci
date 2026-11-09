// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// A live file that registers a scenario for preflight runs ends its
// admission with Admitted: a preflight run lets such a scenario run, and
// without the boundary nothing would stop it before its first effect.
func TestAdmittedScenariosEndTheirAdmission(t *testing.T) {
	t.Parallel()

	root := reporoot.Path(t)
	admitted := 0

	for _, tree := range liveScenarioTrees {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return err
			}

			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}

			text := string(reporoot.ReadFile(t, filepath.ToSlash(rel)))
			registers := strings.Count(text, "RegisterAdmittedScenario(t)")
			boundaries := strings.Count(text, "livetest.Admitted(t)") + strings.Count(text, "\tAdmitted(t)")

			if registers > 0 {
				admitted++
			}

			if registers > 0 && boundaries < registers {
				t.Errorf("%s registers %d admitted scenario(s) and ends %d admission(s) with Admitted", rel, registers, boundaries)
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if admitted == 0 {
		t.Fatal("no live scenario registers for preflight runs; the guard checks nothing")
	}
}
