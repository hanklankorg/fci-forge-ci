// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build linux

package ci

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type canonicalFixture struct {
	env                                                                  map[string]string
	root, engine, binary, metadata, marker, helperMarker, gitMarker, cli string
}

func canonicalWrite(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func canonicalGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("/usr/bin/git", append([]string{"-C", root}, args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newCanonicalFixture(t *testing.T) *canonicalFixture {
	t.Helper()
	if runtime.GOARCH != "amd64" {
		t.Skip("native linux/amd64 unit")
	}
	bootstrap := os.Getenv("FORGE_CI_REVIEWED_BOOTSTRAP")
	if bootstrap == "" {
		bootstrap = "trusted-bootstrap.sh"
	}
	helper, err := os.ReadFile(bootstrap)
	if os.IsNotExist(err) {
		t.Skip("queued trusted-bootstrap dependency absent; set FORGE_CI_REVIEWED_BOOTSTRAP to the frozen reviewed helper for this integration test")
	}
	if err != nil {
		t.Fatal(err)
	}
	f := &canonicalFixture{root: t.TempDir()}
	f.engine = filepath.Join(f.root, "engine")
	for _, dir := range []string{filepath.Join(f.engine, ".engine/bin"), filepath.Join(f.engine, "scripts/ci")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	f.marker = filepath.Join(f.root, "cli-processes")
	f.helperMarker = filepath.Join(f.root, "helper-processes")
	f.cli = "#!/bin/bash\nset -eu\nprintf 'canonical:%s:%s:%s:%s:%s\\n' \"$1\" \"${FORGE_CI_BIN-unset}\" \"${FORGE_CI_PATH-unset}\" \"${FORGE_CI_BINARY_REF-unset}\" \"${FORGE_CI_PLAN-unset}\" >>\"$PROCESS_MARKER\"\n"
	canonicalWrite(t, filepath.Join(f.engine, "actual-trusted-bootstrap.sh"), string(helper), 0o600)
	// Test-only invocation marker delegates to the UNCHANGED reviewed helper.
	canonicalWrite(t, filepath.Join(f.engine, "scripts/ci/trusted-bootstrap.sh"), "#!/bin/bash\nprintf helper >>\"$HELPER_MARKER\"\nexec /bin/bash '"+filepath.Join(f.engine, "actual-trusted-bootstrap.sh")+"' \"$@\"\n", 0o600)
	for _, relative := range []string{"scripts/ci/engine-binary.sh", ".engine/bin/lib-forge-ci.sh"} {
		body, err := os.ReadFile(filepath.Join("../..", relative))
		if err != nil {
			t.Fatal(err)
		}
		canonicalWrite(t, filepath.Join(f.engine, relative), string(body), 0o600)
	}
	canonicalWrite(t, filepath.Join(f.engine, ".engine/bin/forge-ci-linux-amd64.sha256"), fmt.Sprintf("%x  bin/forge-ci\n", sha256.Sum256([]byte(f.cli))), 0o600)
	canonicalWrite(t, filepath.Join(f.engine, "go.mod"), "module example.invalid/inert\n\ngo 1.26.4\n", 0o600)
	canonicalGit(t, f.engine, "init", "-q")
	canonicalGit(t, f.engine, "remote", "add", "origin", "https://github.invalid/admin/engine.git")
	canonicalGit(t, f.engine, "add", ".")
	canonicalGit(t, f.engine, "-c", "user.name=Offline", "-c", "user.email=offline@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "inert fixture")
	commit := canonicalGit(t, f.engine, "rev-parse", "HEAD")
	f.gitMarker = filepath.Join(f.root, "git-processes")
	machinery := filepath.Join(f.root, "repository-local-machinery")
	canonicalWrite(t, machinery, "#!/bin/sh\nprintf invoked >>'"+f.gitMarker+"'\ncat\n", 0o700)
	canonicalGit(t, f.engine, "config", "core.fsmonitor", machinery)
	canonicalGit(t, f.engine, "config", "filter.marker.clean", machinery)
	canonicalGit(t, f.engine, "config", "filter.marker.required", "true")
	canonicalWrite(t, filepath.Join(f.engine, ".gitattributes"), ".engine/bin/* filter=marker\nscripts/ci/* filter=marker\n", 0o600)
	delivery := filepath.Join(f.root, "forge-ci-canonical-100-2")
	f.binary = filepath.Join(delivery, "forge-ci-linux-amd64")
	f.metadata = filepath.Join(f.root, "forge-ci-canonical-metadata.json")
	hostile := filepath.Join(f.root, "hostile")
	if err := os.Mkdir(hostile, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"forge-ci", "git", "sha256sum", "jq", "stat", "gh", "chmod", "uname", "env"} {
		canonicalWrite(t, filepath.Join(hostile, name), "#!/bin/bash\nprintf hostile >>\"$PROCESS_MARKER\"\nexit 99\n", 0o700)
	}
	f.env = map[string]string{
		"PATH": hostile, "HOME": f.root, "ENGINE_CHECKOUT": f.engine, "ENGINE_REPOSITORY": "admin/engine", "ENGINE_COMMIT": commit,
		"ENGINE_ORIGIN": "https://github.invalid/admin/engine", "GITHUB_SERVER_URL": "https://github.invalid",
		"RUNTIME_IMAGE": "registry.invalid/approved@sha256:" + strings.Repeat("a", 64), "GITHUB_RUN_ID": "100", "GITHUB_RUN_ATTEMPT": "2",
		"GITHUB_REPOSITORY_ID": "200", "GITHUB_SHA": strings.Repeat("b", 40), "RUNNER_TEMP": f.root, "DELIVERY_DIRECTORY": delivery,
		"PRODUCER_ARTIFACT_ID": "300", "PRODUCER_ARTIFACT_NAME": "forge-ci-canonical-linux-amd64-100-2-" + commit,
		"PRODUCER_RUN_ID": "100", "PRODUCER_RUN_ATTEMPT": "2", "PRODUCER_ENGINE_REPOSITORY": "admin/engine", "PRODUCER_ENGINE_COMMIT": commit,
		"ARTIFACT_METADATA": f.metadata, "PROCESS_MARKER": f.marker, "HELPER_MARKER": f.helperMarker, "FORGE_CI_BIN": filepath.Join(hostile, "forge-ci"),
		"FORGE_CI_PLAN": "/consumer/plan", "FORGE_CI_PATH": hostile, "FORGE_CI_BINARY_REF": "older-sdk", "SIGNER_STRICT": "0",
		"GITHUB_OUTPUT": filepath.Join(f.root, "outputs"),
	}
	f.writeMetadata(t, nil)
	return f
}

func (f *canonicalFixture) writeMetadata(t *testing.T, mutate func(map[string]any)) {
	t.Helper()
	m := map[string]any{"id": 300, "name": f.env["PRODUCER_ARTIFACT_NAME"], "expired": false, "size_in_bytes": 1024,
		"workflow_run": map[string]any{"id": 100, "repository_id": 200, "head_sha": f.env["GITHUB_SHA"]}}
	if mutate != nil {
		mutate(m)
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	canonicalWrite(t, f.metadata, string(body), 0o600)
}

func (f *canonicalFixture) run(t *testing.T, mode string) ([]byte, error) {
	t.Helper()
	script, err := filepath.Abs("canonical-cli-delivery.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/bash", "--noprofile", "--norc", script, mode)
	for key, value := range f.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, runErr := cmd.CombinedOutput()
	if body, err := os.ReadFile(f.gitMarker); !os.IsNotExist(err) {
		t.Fatalf("repository-local executable Git machinery ran: %q (%v), output %s", body, err, out)
	}
	return out, runErr
}

func TestCanonicalDeliveryReceiver(t *testing.T) {
	for _, scenario := range []string{"verified", "foreign-id", "foreign-run", "foreign-repository", "foreign-source", "stale-attempt", "wrong-producer-ref", "wrong-checkout", "wrong-origin", "wrong-hash", "missing", "symlink", "sidecar", "nested", "oversized-zip", "oversized-file", "modified-helper", "existing-destination"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCanonicalFixture(t)
			if out, err := f.run(t, "prepare"); err != nil {
				t.Fatalf("prepare: %v\n%s", err, out)
			}
			if out, err := f.run(t, "admit"); err != nil {
				t.Fatalf("admit: %v\n%s", err, out)
			}
			canonicalWrite(t, f.binary, f.cli, 0o600) // native action restores mode 0644/0600
			mode := "receive"
			switch scenario {
			case "foreign-id":
				f.writeMetadata(t, func(m map[string]any) { m["id"] = 301 })
			case "foreign-run":
				f.writeMetadata(t, func(m map[string]any) { m["workflow_run"].(map[string]any)["id"] = 101 })
			case "foreign-repository":
				f.writeMetadata(t, func(m map[string]any) { m["workflow_run"].(map[string]any)["repository_id"] = 201 })
			case "foreign-source":
				f.writeMetadata(t, func(m map[string]any) { m["workflow_run"].(map[string]any)["head_sha"] = strings.Repeat("c", 40) })
			case "stale-attempt":
				f.env["PRODUCER_RUN_ATTEMPT"] = "1"
			case "wrong-producer-ref":
				f.env["PRODUCER_ENGINE_COMMIT"] = strings.Repeat("d", 40)
			case "wrong-checkout":
				f.env["ENGINE_COMMIT"] = strings.Repeat("e", 40)
			case "wrong-origin":
				f.env["ENGINE_ORIGIN"] = "https://github.invalid/other/engine"
			case "wrong-hash":
				canonicalWrite(t, f.binary, "#!/bin/bash\nprintf bad >>\"$PROCESS_MARKER\"\n", 0o600)
			case "missing":
				if err := os.Remove(f.binary); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(f.binary); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/bin/false", f.binary); err != nil {
					t.Fatal(err)
				}
			case "sidecar":
				canonicalWrite(t, filepath.Join(filepath.Dir(f.binary), "sidecar"), "extra", 0o600)
			case "nested":
				if err := os.Mkdir(filepath.Join(filepath.Dir(f.binary), "nested"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "oversized-zip":
				f.writeMetadata(t, func(m map[string]any) { m["size_in_bytes"] = 134217729 })
			case "oversized-file":
				if err := os.Truncate(f.binary, 134217729); err != nil {
					t.Fatal(err)
				}
			case "modified-helper":
				canonicalWrite(t, filepath.Join(f.engine, "scripts/ci/trusted-bootstrap.sh"), "printf helper >>\"$HELPER_MARKER\"\n", 0o600)
			case "existing-destination":
				mode = "prepare"
			}
			out, err := f.run(t, mode)
			if scenario == "verified" {
				if err != nil {
					t.Fatalf("receive: %v\n%s", err, out)
				}
				body, err := os.ReadFile(f.marker)
				if err != nil || string(body) != "canonical:--version:unset:unset:unset:unset\n" {
					t.Fatalf("wrong CLI process: %q (%v)", body, err)
				}
				helper, err := os.ReadFile(f.helperMarker)
				if err != nil || string(helper) != "helper" {
					t.Fatalf("wrong helper process: %q (%v)", helper, err)
				}
			} else {
				if err == nil {
					t.Fatalf("accepted %s: %s", scenario, out)
				}
				for _, marker := range []string{f.marker, f.helperMarker} {
					if body, err := os.ReadFile(marker); !os.IsNotExist(err) {
						t.Fatalf("process before admission: %q (%v), output %s", body, err, out)
					}
				}
			}
		})
	}
}

func TestCanonicalDeliveryProducer(t *testing.T) {
	f := newCanonicalFixture(t)
	cli := filepath.Join(f.root, "inert-cli")
	canonicalWrite(t, cli, f.cli, 0o600)
	compiler := filepath.Join(f.root, "approved-inert-go")
	// Exercise the EXISTING recipe/toolchain check without compiling the CLI or
	// spending live-lane CPU. This is not a qualification/reproducibility claim.
	canonicalWrite(t, compiler, "#!/bin/bash\nset -eu\nif [[ $1 == env ]]; then printf 'go1.26.4\\n'; exit; fi\nwhile [[ $# -gt 0 ]]; do if [[ $1 == -o ]]; then cp '"+cli+"' \"$2\"; exit; fi; shift; done\nexit 99\n", 0o700)
	f.env["GO"] = compiler
	if out, err := f.run(t, "build"); err != nil {
		t.Fatalf("canonical recipe: %v\n%s", err, out)
	}
	outputs, err := os.ReadFile(f.env["GITHUB_OUTPUT"])
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"run-id=100\n", "run-attempt=2\n", "artifact-name=" + f.env["PRODUCER_ARTIFACT_NAME"] + "\n", "engine-commit=" + f.env["ENGINE_COMMIT"] + "\n"} {
		if !strings.Contains(string(outputs), line) {
			t.Fatalf("original producer context missing %q: %s", line, outputs)
		}
	}
	for _, marker := range []string{f.marker, f.helperMarker} {
		if body, err := os.ReadFile(marker); !os.IsNotExist(err) {
			t.Fatalf("producer invoked helper/CLI: %q (%v)", body, err)
		}
	}
}

func TestCanonicalDeliveryWorkflowOrdering(t *testing.T) {
	body, err := os.ReadFile("../../.github/workflows/canonical-cli-bootstrap.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	for _, forbidden := range []string{"id-token:", "write-all", ": write", "workflow_dispatch:", "pull_request:", "secrets:"} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("not a read-only protected reusable unit: %s", forbidden)
		}
	}
	_, receiver, ok := strings.Cut(workflow, "  canonical-receive:\n")
	if !ok {
		t.Fatal("missing receiver")
	}
	last := -1
	for _, boundary := range []string{"needs: canonical-build", "Separate approved engine checkout", "canonical-cli-delivery.sh\" prepare", "gh api --hostname", "canonical-cli-delivery.sh\" admit", "actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c", "artifact-ids: ${{ needs.canonical-build.outputs.artifact-id }}", "canonical-cli-delivery.sh\" receive"} {
		n := strings.Index(receiver, boundary)
		if n <= last {
			t.Fatalf("missing/out-of-order boundary: %s", boundary)
		}
		last = n
	}
	for _, forbidden := range []string{"environment:", "secrets:", "id-token:", "uses: ./", "name: engine-cli", "run-id: ${{", "artifact download"} {
		if strings.Contains(receiver, forbidden) {
			t.Fatalf("unsafe bootstrap receiver: %s", forbidden)
		}
	}
	if !strings.Contains(workflow, "artifact-id: ${{ steps.upload.outputs.artifact-id }}") || !strings.Contains(workflow, "run-attempt: ${{ steps.build.outputs.run-attempt }}") {
		t.Fatal("not forwarding original producer outputs")
	}
}
