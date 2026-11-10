// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// preparedImages is a campaign.env with every image a case can name.
var preparedImages = map[string]string{
	"JOB_IMAGE_GO": "lab/go@sha256:01", "JOB_IMAGE_JAVA": "lab/java@sha256:02",
	"JOB_IMAGE_JAVASCRIPT_TYPESCRIPT": "lab/js@sha256:03", "JOB_IMAGE_RUST": "lab/rust@sha256:04",
	"KANIKO_JOB_IMAGE": "lab/kaniko@sha256:05",
}

// Temporary scaffolding for the migration from the cases files to
// ForgejoPreparedCases: each lane is the consumer rows of its former file,
// row for row, with the same demo, event and settings. It goes with the files.
func TestForgejoPreparedCasesMatchCasesFiles(t *testing.T) {
	t.Parallel()

	for lane, file := range map[string]string{
		LaneLifecycle: "lifecycle-forgejo.cases", LaneSnapshot: "snapshot-forgejo.cases",
		LaneReleaseVariants: "release-variants-forgejo.cases", LanePrivate: "k3s-private-forgejo.cases",
	} {
		t.Run(lane, func(t *testing.T) {
			t.Parallel()

			body, err := os.ReadFile(filepath.Join(reporoot.Path(t), "test/live/campaigns", file))
			if err != nil {
				t.Fatal(err)
			}

			var want []string

			for line := range strings.SplitSeq(string(body), "\n") {
				words := strings.Fields(line)
				if len(words) < 3 || words[0] != caseConsumer {
					continue
				}

				for index, word := range words[3:] {
					if key, name, isRef := strings.Cut(word, "=@"); isRef {
						words[3+index] = key + "=" + preparedImages[name]
					}
				}

				slices.Sort(words[3:])
				want = append(want, strings.Join(words[1:], " "))
			}

			var got []string

			for _, c := range ForgejoPreparedCases {
				if c.Lane != lane {
					continue
				}

				settings, err := c.Settings(preparedImages)
				if err != nil {
					t.Fatal(err)
				}

				slices.Sort(settings)
				got = append(got, strings.Join(append([]string{c.Demo, c.Event}, settings...), " "))
			}

			if len(want) == 0 || !slices.Equal(got, want) {
				t.Fatalf("the lane's cases:\n%s\nthe file's consumer rows:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

func TestForgejoPreparedCaseNamesAreUnique(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, c := range ForgejoPreparedCases {
		if seen[c.Name()] {
			t.Errorf("two cases are named %s", c.Name())
		}

		seen[c.Name()] = true
	}
}

func TestPreparedCaseSettingsNameTheMissingImage(t *testing.T) {
	t.Parallel()

	kaniko := PreparedCase{Lane: LanePrivate, Demo: "hello-go-cli", Event: TrialPullRequest, JobImage: flavorGo, Builder: ImageBuilderKaniko}

	for _, missing := range []string{"JOB_IMAGE_GO", "KANIKO_JOB_IMAGE"} {
		prepared := map[string]string{"JOB_IMAGE_GO": "lab/go@sha256:01", "KANIKO_JOB_IMAGE": "lab/kaniko@sha256:05"}
		delete(prepared, missing)

		_, err := kaniko.Settings(prepared)
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "has no "+missing+";") {
			t.Errorf("without %s: %v", missing, err)
		}
	}
}

func TestPreparedForgejoEnvironment(t *testing.T) {
	t.Parallel()

	got := PreparedForgejoEnvironment(map[string]string{
		"CAMPAIGN_COMMIT": "0123", "JOB_IMAGE_GO": "lab/go@sha256:01",
		"FORGE_CI_LIVE_SIGNER_IMAGE": "lab/signer@sha256:aa", "FORGE_CI_LIVE_KANIKO_JOB_IMAGE_ARCHIVE": "/prepared/kaniko.tar",
	})
	// The Buildah archive was not prepared: it is there and empty, which
	// clears a value the caller's environment may hold.
	buildah, cleared := got[GitLabSnapshotBuilderArchiveEnv(ImageBuilderBuildah)]
	if len(got) != 3 || got["FORGE_CI_LIVE_SIGNER_IMAGE"] != "lab/signer@sha256:aa" || got[KanikoJobImageArchiveEnv] != "/prepared/kaniko.tar" || !cleared || buildah != "" {
		t.Fatalf("the environment is %v", got)
	}
}
