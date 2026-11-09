// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"reflect"
	"strings"
	"testing"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

func TestGitLabReleaseRunnerRoles(t *testing.T) {
	for _, tc := range []struct {
		name            string
		images, exports bool
		want            map[string][]string
	}{
		{"files", false, false, map[string][]string{"Build: release files": {"ordinary"}, "Check: consumer checks": {"build"}}},
		{"images", true, false, map[string][]string{"Build: release files": {"ordinary"}, "Check: consumer checks": {"build"}, "Images: build amd64": {"build"}}},
		{"exports", false, true, map[string][]string{"Build: release files": {"ordinary"}, "Check: consumer checks": {"build"}, "Build: export amd64": {"build"}}},
		{"images and exports", true, true, map[string][]string{"Build: release files": {"ordinary"}, "Check: consumer checks": {"build"}, "Images: build amd64": {"build"}, "Build: export amd64": {"build"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := gitLabReleaseRunnerRoles([]string{"build"}, []string{"ordinary"}, tc.images, tc.exports); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("roles: got %v, want %v", got, tc.want)
			}
		})
	}
}

// Pure oracle tests: no target, credentials, or provider calls are needed.
func TestGitLabReleaseRunnerRoutingOracle(t *testing.T) {
	for _, images := range []bool{false, true} {
		for _, exports := range []bool{false, true} {
			expected := gitLabReleaseRunnerRoles([]string{"privileged"}, []string{"checks"}, images, exports)
			jobs := []*gitlabapi.Job{}

			for name, tags := range expected {
				runner := int64(3)
				if len(tags) != 0 {
					runner = 1
					if tags[0] == "checks" {
						runner = 2
					}
				}

				jobs = append(jobs, &gitlabapi.Job{Name: name, TagList: tags, Status: "success", Runner: gitlabapi.JobRunner{ID: runner}})
			}

			jobs = append(jobs, &gitlabapi.Job{Name: "Sign: release", Status: "success", Runner: gitlabapi.JobRunner{ID: 3}})
			if err := validateGitLabReleaseRunnerRouting(jobs, expected); err != nil {
				t.Fatalf("images=%v exports=%v: %v", images, exports, err)
			}
		}
	}

	cases := []struct {
		name string
		job  gitlabapi.Job
		want string
	}{
		{"wrong builder tags", gitlabapi.Job{Name: "Images: build amd64", TagList: []string{"checks"}, Status: "success", Runner: gitlabapi.JobRunner{ID: 2}}, "requested tags"},
		{"unexecuted builder", gitlabapi.Job{Name: "Images: build amd64", TagList: []string{"privileged"}, Status: "skipped"}, "requested tags"},
		{"failed builder", gitlabapi.Job{Name: "Images: build amd64", TagList: []string{"privileged"}, Status: "failed", Runner: gitlabapi.JobRunner{ID: 1}}, "requested tags"},
		{"no builder runner", gitlabapi.Job{Name: "Images: build amd64", TagList: []string{"privileged"}, Status: "success"}, "requested tags"},
		{"missing builder", gitlabapi.Job{Name: "Summary: release", Runner: gitlabapi.JobRunner{ID: 3}}, "missing successful jobs"},
		{"tag leakage", gitlabapi.Job{Name: "Sign: release", TagList: []string{"privileged"}, Runner: gitlabapi.JobRunner{ID: 1}}, "unrelated job"},
		{"runner leakage", gitlabapi.Job{Name: "Sign: release", Runner: gitlabapi.JobRunner{ID: 1}}, "tagged capability runner"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expected := gitLabReleaseRunnerRoles([]string{"privileged"}, []string{"checks"}, true, false)

			jobs := []*gitlabapi.Job{
				{Name: "Build: release files", TagList: []string{"checks"}, Status: "success", Runner: gitlabapi.JobRunner{ID: 2}},
				{Name: "Check: consumer checks", TagList: []string{"privileged"}, Status: "success", Runner: gitlabapi.JobRunner{ID: 1}},
				&tc.job,
			}
			if tc.job.Name != "Images: build amd64" && tc.name != "missing builder" {
				jobs = append(jobs, &gitlabapi.Job{Name: "Images: build amd64", TagList: []string{"privileged"}, Status: "success", Runner: gitlabapi.JobRunner{ID: 1}})
			}

			if err := validateGitLabReleaseRunnerRouting(jobs, expected); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}

	for _, name := range []string{"Images: build amd64", "Build: export amd64"} {
		for _, tagged := range []bool{false, true} {
			// Untagged image and export jobs beside tagged ordinary ones.
			expected := gitLabReleaseRunnerRoles(nil, []string{"privileged"}, true, true)

			jobs := []*gitlabapi.Job{
				{Name: "Build: release files", TagList: []string{"privileged"}, Status: "success", Runner: gitlabapi.JobRunner{ID: 1}},
				{Name: "Check: consumer checks", Status: "success", Runner: gitlabapi.JobRunner{ID: 2}},
				{Name: "Images: build amd64", Status: "success", Runner: gitlabapi.JobRunner{ID: 2}},
				{Name: "Build: export amd64", Status: "success", Runner: gitlabapi.JobRunner{ID: 2}},
			}
			for _, job := range jobs {
				if job.Name == name {
					job.Runner.ID = 1
					if tagged {
						job.TagList = []string{"privileged"}
					}
				}
			}

			if err := validateGitLabReleaseRunnerRouting(jobs, expected); err == nil {
				t.Fatalf("untagged role %q accepted capability leakage (tagged=%v)", name, tagged)
			}
		}
	}
}
