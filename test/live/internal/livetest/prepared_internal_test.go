// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

func TestCheckPreparedSource(t *testing.T) {
	t.Parallel()

	const commit = "0123456789abcdef0123456789abcdef01234567"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CampaignEnvFile), []byte("CAMPAIGN_COMMIT="+commit+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := CheckPreparedSource(dir, commit, ""); err != nil {
		t.Fatalf("the prepared commit, clean, is refused: %v", err)
	}

	for name, tc := range map[string]struct{ dir, head, changes string }{
		"another commit":    {dir, "89abcdef0123456789abcdef0123456789abcdef", ""},
		"a changed file":    {dir, commit, " M scripts/ci/run-live-tests.sh\n"},
		"an untracked file": {dir, commit, "?? notes.txt\n"},
		"a relative dir":    {"prepared", commit, ""},
		"nothing prepared":  {t.TempDir(), commit, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := CheckPreparedSource(tc.dir, tc.head, tc.changes); err == nil {
				t.Fatal("accepted")
			} else if name != "nothing prepared" && !errors.Is(err, errs.ErrValidation) {
				t.Fatalf("not a validation error: %v", err)
			}
		})
	}
}
