// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	forgecli "github.com/diggsweden/forge-ci/internal/cli"
	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"github.com/stretchr/testify/require"
)

// The child runs the actual CLI command tree, never a provider. Only external
// cryptographic mechanisms are inert executable fixtures; no real keys exist.
func TestClassifiedLeafCLIProcess(t *testing.T) {
	if os.Getenv("CLASSIFIED_CLI_CHILD") != "1" {
		return
	}

	for i, arg := range os.Args {
		if arg == "--" {
			err := forgecli.New(forgecli.BuildInfo{}).Run(t.Context(), append([]string{"forge-ci", "--provider", "local", "--runner", "github"}, os.Args[i+1:]...))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			os.Exit(0)
		}
	}

	t.Fatal("missing command separator")
}

func TestClassifiedLeafNativeCLIUnsignedSignFinalizePublish(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	require.NoError(t, os.Mkdir(bin, 0o700))

	executable, err := os.Executable()
	require.NoError(t, err)

	write := func(path, body string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o700))
	}
	write(filepath.Join(bin, "forge-ci"), "#!/bin/bash\nexport CLASSIFIED_CLI_CHILD=1\nexec "+strconv.Quote(executable)+" -test.run='^TestClassifiedLeafCLIProcess$' -- \"$@\"\n")
	write(filepath.Join(bin, "gpg"), `#!/bin/bash
set -eu
case " $* " in
  *" --import "*) cat >/dev/null ;;
  *" --list-secret-keys "*) printf 'fpr:::::::::ABCDEF:\n' ;;
  *" --detach-sign "*)
    while [[ $# -gt 0 ]]; do
      if [[ "$1" == --output ]]; then printf 'inert package signature' >"$2"; break; fi
      shift
    done ;;
  *) exit 91 ;;
esac
`)
	write(filepath.Join(bin, "gpgconf"), "#!/bin/bash\nexit 0\n")
	write(filepath.Join(bin, "cosign"), `#!/bin/bash
set -eu
case "$1" in
  public-key) printf 'inert public comparison bytes\n' ;;
  sign-blob)
    while [[ $# -gt 0 ]]; do
      if [[ "$1" == --bundle ]]; then printf 'inert bundle, not crypto evidence' >"$2"; break; fi
      shift
    done ;;
  *) exit 92 ;;
esac
`)

	unsigned := filepath.Join(root, "classified-product")
	for name, body := range map[string]string{
		"product-v1.2.3-arm64.tar.gz": "opaque archive\x00\xff\n", "nested/product_arm64.deb": "opaque package\n",
		"product.cdx.json": "{ \"serialNumber\" : \"pinned-producer\" }\n", "notes.md": "# Producer release notes\n",
	} {
		write(filepath.Join(unsigned, "dist", name), body)
	}

	env := []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + root, "RUNNER_TEMP=" + root, "GITHUB_OUTPUT=" + filepath.Join(root, "output"), "FORGE_CI_PLAN=", "CHECKSUMS_FILE=", "GITHUB_ACTIONS=true", "GITHUB_SERVER_URL=https://github.invalid", "GITHUB_REPOSITORY=owner/product", "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=2", "GITHUB_WORKFLOW_REF=owner/product/.github/workflows/release.yml@refs/tags/release-request/v1.2.3", "GITHUB_REF_NAME=release-request/v1.2.3", "GITHUB_SHA=" + strings.Repeat("a", 40), "PREPARED_SHA=" + strings.Repeat("b", 40), "SOURCE_DATE_EPOCH=1234", "TAG=v1.2.3", "NOTES_PATH=dist/notes.md", "GOMAXPROCS=2", "GORACE=atexit_sleep_ms=0"}
	run := func(directory, body string, extra ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", body)

		cmd.Dir, cmd.Env = directory, append(append([]string{}, env...), extra...)
		out, runErr := cmd.CombinedOutput()
		require.NoError(t, runErr, string(out))

		return string(out)
	}
	run(unsigned, `forge-ci release files declare --asset dist/product-v1.2.3-arm64.tar.gz --asset dist/nested/product_arm64.deb --sbom dist/product.cdx.json --evidence-file dist/notes.md --checksums-file dist/original-checksums.txt`)

	original := map[string][]byte{}
	for _, name := range []string{"dist/product-v1.2.3-arm64.tar.gz", "dist/nested/product_arm64.deb", "dist/product.cdx.json", "dist/notes.md", "dist/original-checksums.txt"} {
		original[name], err = os.ReadFile(filepath.Join(unsigned, name))
		require.NoError(t, err)
	}

	source := filepath.Join(root, "classified-source")
	require.NoError(t, os.Mkdir(source, 0o700))
	require.NoError(t, os.Rename(unsigned, filepath.Join(source, "fixture")))
	write(filepath.Join(source, "release-artifacts", "source-only.deb"), "not a producer upload")
	write(filepath.Join(source, "produce"), `#!/bin/bash
set -eu
test -z "${GITHUB_TOKEN+x}${GPG_SIGNING_KEY+x}${COSIGN_KEY+x}"
test -z "${GH_TOKEN+x}${FORGE_CI_BIN+x}${FORGE_CI_PLAN+x}${GITHUB_ENV+x}${CARGO_REGISTRIES_CRATES_IO_TOKEN+x}"
test "$HOME" = "${FORGE_CI_CLASSIFIED_OUTPUT%/*}/classified-product-home"
test "$PATH" = /usr/local/go/bin:/usr/local/cargo/bin:/usr/local/bin:/usr/bin:/bin
test "$RUSTUP_HOME" = /usr/local/rustup
test "$CARGO_HOME" = "$HOME/.cargo"
cp -a fixture/dist "$FORGE_CI_CLASSIFIED_OUTPUT/dist"
`)
	write(filepath.Join(root, "forge-ci-approved-engine/scripts/ci/trusted-runtime-tools.sh"), string(reporoot.ReadFile(t, "scripts/ci/trusted-runtime-tools.sh")))
	run(root, classifiedLeafStep(t, "producer", "Run administrator-selected").Run, "PRODUCER_PATH=produce", "GITHUB_TOKEN=must-not-reach-product", "COSIGN_KEY=must-not-reach-product", "GH_TOKEN=must-not-reach-product", "CARGO_REGISTRIES_CRATES_IO_TOKEN=must-not-reach-product", "FORGE_CI_BIN=/host/old-cli", "CARGO_HOME=/host/cache", "RUSTUP_HOME=/host/toolchain")
	// Actual producer handoff block, including command-substitution fail-fast.
	run(root, classifiedLeafStep(t, "producer", "Admit exact unsigned").Run)
	output, err := os.ReadFile(filepath.Join(root, "output"))
	require.NoError(t, err)

	digest := strings.TrimSpace(strings.TrimPrefix(string(output), "digest="))
	run(root, `forge-ci release validate-dist --classified-files --dist-dir "$RUNNER_TEMP/classified-product" --stage-dir "$RUNNER_TEMP/classified-admitted" --expected-digest "`+digest+`"`)
	run(root, classifiedLeafStep(t, "sign", "Independently GPG").Run, "GPG_SIGNING_KEY=aW5lcnQ=", "GPG_SIGNING_PASSWORD=inert-pass", "PACKAGE_FINGERPRINT=ABCDEF")

	public := "inert public comparison bytes\n"
	run(root, classifiedLeafStep(t, "sign", "Bind Cosign").Run, "COSIGN_KEY=aW5lcnQ=", "COSIGN_PASSWORD=inert-pass", "TRUSTED_PUBLIC_KEY="+public, fmt.Sprintf("TRUSTED_PUBLIC_DIGEST=%x", sha256.Sum256([]byte(public))), "FORGE_CI_COSIGN_TRANSPARENCY=none")
	run(root, classifiedLeafStep(t, "sign", "Finalize native").Run)
	output, err = os.ReadFile(filepath.Join(root, "output"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	signedDigest := strings.TrimPrefix(lines[len(lines)-1], "digest=")
	run(root, `forge-ci release validate-dist --dist-dir "$RUNNER_TEMP/classified-admitted" --stage-dir "$RUNNER_TEMP/classified-signed" --manifest-root . --expected-digest "`+signedDigest+`"`)

	for name, body := range original {
		actual, readErr := os.ReadFile(filepath.Join(root, "classified-signed", name))
		require.NoError(t, readErr)
		require.Equal(t, body, actual, "producer bytes and names survive independent roles: "+name)
	}

	publish := classifiedLeafStep(t, "publish", "Publish only").Run
	preview := run(root, strings.Replace(publish, "--strategy reconcile", "--strategy reconcile --dry-run", 1), `RELEASE_PLAN={"context":{"release_policy":"stable"}}`)
	require.Contains(t, preview, "product-v1.2.3-arm64.tar.gz")
	require.Contains(t, preview, "product_arm64.deb.sig")
	require.Contains(t, preview, "product.cdx.json.bundle")
	require.Contains(t, preview, "slsa-provenance.intoto.json.bundle")
	require.NotContains(t, preview, "source-only.deb")

	for _, scenario := range []string{"signed-digest-tamper", "missing-sidecar"} {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join(root, "classified-signed")
			body := `forge-ci release files validate --manifest dist/release-files.json --dist-dir dist`

			path := filepath.Join(dir, "dist", "product.cdx.json.bundle")
			if scenario == "signed-digest-tamper" {
				write(path, "changed signed bytes")

				body = `forge-ci release validate-dist --dist-dir . --manifest-root . --expected-digest "` + signedDigest + `"`
			} else {
				require.NoError(t, os.Remove(path))
			}

			cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", body+"\nprintf forbidden > \"$RUNNER_TEMP/publication-effect\"\n")
			cmd.Dir, cmd.Env = dir, env
			_, runErr := cmd.CombinedOutput()
			require.Error(t, runErr)
			require.NoFileExists(t, filepath.Join(root, "publication-effect"))
		})
	}
}

func classifiedLeafPlanEnv(t *testing.T, kind projecttype.Type) []string {
	t.Helper()

	artifact := config.Artifact{Name: "product", ProjectType: kind, WorkingDirectory: ".", SBOMs: "none"}
	goMap, cargoMap := `{}`, `{}`

	if kind == projecttype.Go {
		artifact.Go = &config.GoConfig{BuildMode: config.GoBuildModeArtifactFirst}
		goMap = `{"product":{"packages":["./test/policy"],"gate":"policy"}}`
	} else {
		artifact.Cargo = &config.CargoConfig{BuildMode: config.CargoBuildModeArtifactFirst}
		cargoMap = `{"product":{"runs":[{"kind":"package-test","package":"policy","name":"workflow_policy","gate":"policy"}]}}`
	}

	cfg := pipeline.NewConfigPlan(&config.Config{Artifacts: []config.Artifact{artifact}})
	plan, err := pipeline.NewReleasePlan(pipeline.ReleasePlanInput{ConfigPlan: cfg, RepositoryVisibility: "private", Branch: "main", RefName: "v1.2.3", ReleasePublisher: "github-cli", ReleaseRequireAllowlistedSigner: true, ReleaseSignArtifacts: true, ReleaseSBOMs: "none", ChangelogCreator: "git-cliff"})
	require.NoError(t, err)
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	planJSON, err := json.Marshal(plan)
	require.NoError(t, err)

	return []string{"CONFIG_PLAN=" + string(cfgJSON), "RELEASE_PLAN=" + string(planJSON), "GO_SELECTIONS=" + goMap, "CARGO_SELECTIONS=" + cargoMap, "PRODUCER_FILE_ONLY=true", "RELEASE_SBOMS=none", "TAG=v1.2.3", "ADMIN_APPLICATION_TESTS=false"}
}

func TestClassifiedLeafNativePolicyIsMandatory(t *testing.T) {
	for _, kind := range []projecttype.Type{projecttype.Go, projecttype.Cargo} {
		for _, fails := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fails=%t", kind, fails), func(t *testing.T) {
				root := t.TempDir()
				bin := filepath.Join(root, "bin")
				source := filepath.Join(root, "classified-source")
				require.NoError(t, os.MkdirAll(filepath.Join(source, "test", "policy"), 0o700))
				require.NoError(t, os.Mkdir(bin, 0o700))

				executable, err := os.Executable()
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(bin, "forge-ci"), []byte("#!/bin/bash\nexport CLASSIFIED_CLI_CHILD=1\nexec "+strconv.Quote(executable)+" -test.run='^TestClassifiedLeafCLIProcess$' -- \"$@\"\n"), 0o700))

				goTool, err := exec.LookPath("go")
				require.NoError(t, err)
				cargoTool, err := exec.LookPath("cargo")
				require.NoError(t, err)
				home, err := os.UserHomeDir()
				require.NoError(t, err)

				cache := os.Getenv("GOCACHE")
				if cache == "" {
					cacheHome, cacheErr := os.UserCacheDir()
					require.NoError(t, cacheErr)

					cache = filepath.Join(cacheHome, "go-build")
				}

				if kind == projecttype.Go {
					require.NoError(t, os.WriteFile(filepath.Join(source, "go.mod"), []byte("module example.invalid/product\n\ngo 1.24\n"), 0o600))

					body := "package policy\nimport \"testing\"\nfunc TestPolicy(t *testing.T) {"
					if fails {
						body += "t.Fatal(\"mandatory policy refusal\")"
					}

					body += "}\n"
					require.NoError(t, os.WriteFile(filepath.Join(source, "test", "policy", "policy_test.go"), []byte(body), 0o600))
				} else {
					require.NoError(t, os.Mkdir(filepath.Join(source, "tests"), 0o700))
					require.NoError(t, os.WriteFile(filepath.Join(source, "Cargo.toml"), []byte("[package]\nname = \"policy\"\nversion = \"1.2.3\"\nedition = \"2021\"\n[[test]]\nname = \"workflow_policy\"\npath = \"tests/policy.rs\"\n"), 0o600))
					require.NoError(t, os.WriteFile(filepath.Join(source, "Cargo.lock"), []byte("version = 4\n[[package]]\nname = \"policy\"\nversion = \"1.2.3\"\n"), 0o600))
					require.NoError(t, os.WriteFile(filepath.Join(source, "tests", "policy.rs"), []byte(fmt.Sprintf("#[test]\nfn policy() { assert!(%t); }\n", !fails)), 0o600))
				}

				body := classifiedLeafStep(t, "policy", "Admit trusted").Run + "\n" + classifiedLeafStep(t, "policy", "Validate complete test").Run + "\nprintf allowed > \"$RUNNER_TEMP/producer-effect\"\n"
				cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", body)
				cmd.Dir = root
				cmd.Env = append([]string{"PATH=" + bin + ":" + filepath.Dir(goTool) + ":" + filepath.Dir(cargoTool) + ":/usr/bin:/bin", "HOME=" + home, "RUNNER_TEMP=" + root, "GITHUB_OUTPUT=" + filepath.Join(root, "output"), "GOMAXPROCS=2", "GORACE=atexit_sleep_ms=0", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOCACHE=" + cache, "GOTMPDIR=" + os.Getenv("GOTMPDIR"), "TMPDIR=" + os.Getenv("TMPDIR"), "CARGO_NET_OFFLINE=true", "CARGO_BUILD_JOBS=1"}, classifiedLeafPlanEnv(t, kind)...)

				out, runErr := cmd.CombinedOutput()
				if fails {
					require.Error(t, runErr, string(out))
					require.NoFileExists(t, filepath.Join(root, "producer-effect"))
				} else {
					require.NoError(t, runErr, string(out))
					require.FileExists(t, filepath.Join(root, "producer-effect"))
				}
			})
		}
	}
}
