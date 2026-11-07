// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build linux

package ci

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Only this inert payload translates the received fixed image PATH into an
// owned filesystem. Production helper/verification bytes are not rewritten.
const runtimeToolLookupFixture = `
[[ "$PATH" == /usr/local/go/bin:/usr/local/cargo/bin:/usr/local/bin:/usr/bin:/bin ]]
[[ "$RUSTUP_HOME" == /usr/local/rustup && "$CARGO_HOME" == "$HOME/.cargo" ]]
IFS=: read -r -a directories <<<"$PATH"
for tool in gh go cargo rustc cosign git-cliff; do
  found=false
  for directory in "${directories[@]}"; do
    candidate="$TEST_IMAGE_ROOT$directory/$tool"
    if [[ -x "$candidate" ]]; then "$candidate"; found=true; break; fi
  done
  [[ "$found" == true ]]
done
`

func TestRuntimeToolMetadataLookup(t *testing.T) {
	helper, err := filepath.Abs("trusted-runtime-tools.sh")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()

	imageTool := filepath.Join(root, "image/usr/local/bin/gh")
	if err := os.MkdirAll(filepath.Dir(imageTool), 0o700); err != nil {
		t.Fatal(err)
	}

	canonicalWrite(t, imageTool, "#!/bin/bash\nprintf '%s\\n' \"$*\"\n", 0o700)
	// Mock only exec's filesystem boundary to represent immutable /usr/local
	// without modifying the host. Run the actual helper's assignments/refusal.
	const execute = `
exec() {
  [[ "$PATH" == /usr/local/go/bin:/usr/local/cargo/bin:/usr/local/bin:/usr/bin:/bin ]]
  IFS=: read -r -a directories <<<"$PATH"
  for directory in "${directories[@]}"; do
    if [[ -x "$TEST_IMAGE_ROOT$directory/$1" ]]; then
      local tool="$TEST_IMAGE_ROOT$directory/$1"
      shift
      "$tool" "$@"
      return
    fi
  done
  return 69
}
source "$1" gh api --hostname approved.invalid repos/owner/product/actions/artifacts/123
`

	cmd := exec.CommandContext(t.Context(), "/bin/bash", "--noprofile", "--norc", "-peu", "-o", "pipefail", "-c", execute, "metadata", helper)
	cmd.Env = []string{"HOME=" + root, "PATH=/consumer/hostile", "TEST_IMAGE_ROOT=" + filepath.Join(root, "image"), "TRUSTED_TOOL_PATH=/consumer/hostile"}

	out, runErr := cmd.CombinedOutput()
	if runErr != nil || string(out) != "api --hostname approved.invalid repos/owner/product/actions/artifacts/123\n" {
		t.Fatalf("metadata tool lookup: %v, %s", runErr, out)
	}
}

func TestRuntimeToolHelperRejectsImplicitCLI(t *testing.T) {
	for _, args := range [][]string{nil, {"forge-ci", "--version"}} {
		cmd := exec.CommandContext(t.Context(), "/bin/bash", append([]string{"-p", "trusted-runtime-tools.sh"}, args...)...)
		cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=/consumer/hostile"}

		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "forge-ci must be verified and absolute") {
			t.Fatalf("implicit CLI accepted: %v: %s", err, out)
		}
	}
}

func TestCanonicalDeliveryRejectsUnadmittedToolHelper(t *testing.T) {
	fixture := newCanonicalFixture(t)
	canonicalWrite(t, filepath.Join(fixture.engine, "scripts/ci/trusted-runtime-tools.sh"), "printf forbidden >>\"$PROCESS_MARKER\"\n", 0o600)

	if out, err := fixture.run(t, "prepare"); err == nil {
		t.Fatalf("modified tool helper passed before metadata: %s", out)
	}

	if _, err := os.Stat(fixture.marker); !os.IsNotExist(err) {
		t.Fatal("modified helper or CLI ran")
	}
}
