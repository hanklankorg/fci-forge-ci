// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package release

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	apprelease "github.com/diggsweden/forge-ci/internal/app/release"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// distDigestCmd exposes the release hand-off digest contract used by
// cross-job build-to-sign boundaries. It intentionally differs from `artifact
// digest`, which has a separate content-digest scheme.
func distDigestCmd() *cli.Command {
	return &cli.Command{
		Name:  "dist-digest",
		Usage: "print the current release dist/ hand-off digest",
		Description: `Computes the digest used by release validate-dist: the SHA-256 of
the sorted sha256sum manifest for every regular file under --dist-dir. Producer
and verifier use this canonical algorithm at the reusable-workflow hand-off.

Set --manifest-root to override the path prefix written into the manifest; use
--manifest-root . for artifact contents that are staged under different
directory names by producer and verifier.

With --classified-files, --dist-dir is a file-only workspace containing
dist/release-files.json and exactly its declared unsigned files. Admit the
classified inventory before hashing; preserve all producer bytes and names.
This mode fixes the manifest prefix to '.' and cannot be combined with
--canonical-payload or an explicit --manifest-root. It establishes integrity,
not source or artifact authorization.

EXAMPLE:
   forge-ci release dist-digest --dist-dir dist`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "canonical-payload", Usage: "admit a workspace containing dist/ and optional premade-sboms/; use the stable '.' manifest prefix"},
			&cli.BoolFlag{Name: "classified-files", Usage: "admit a closed unsigned file-only workspace with dist/release-files.json; fixes manifest prefix to '.'; not source authorization"},
			&cli.StringFlag{Name: flagDistDir, Value: defaultDistDir, Usage: "directory whose release hand-off digest is printed"},
			&cli.StringFlag{Name: "manifest-root", Usage: "path prefix written into the sha256sum manifest (defaults to --dist-dir)"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if err := validateDistPayloadMode(cmd); err != nil {
				return err
			}

			var (
				digest string
				err    error
			)

			switch {
			case cmd.Bool("classified-files"):
				digest, err = apprelease.ClassifiedFilePayloadDigest(cmd.String(flagDistDir))
			case cmd.Bool("canonical-payload"):
				digest, err = apprelease.CanonicalPayloadDigest(cmd.String(flagDistDir))
			default:
				digest, err = apprelease.DistDigestWithManifestRoot(cmd.String(flagDistDir), cmd.String("manifest-root"))
			}

			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(os.Stdout, digest)

			return nil
		},
	}
}

// verifyDistCmd checks that a dist/ tree handed from the (keyless) build job to
// the signing job is structurally safe and byte-identical to what was built,
// refusing to sign tampered artifacts.
func verifyDistCmd() *cli.Command {
	return &cli.Command{
		Name:  "validate-dist",
		Usage: "validate a dist/ tree is structurally safe and matches an expected digest (cross-job integrity)",
		Description: `Rejects a dist/ that is a symlink, contains symlinks, contains
non-regular entries, or has control characters in any path, then
recomputes its canonical digest and fails on mismatch. The digest
algorithm is shared with release dist-digest.

With --stage-dir, copy into private staging, verify those copied bytes and install
them into a new destination only on success. --digest-file reads the producer's
bare lowercase SHA-256 recording and is exclusive with --expected-digest.

Set --manifest-root to override the path prefix written into the manifest during
digest recomputation; use --manifest-root . for artifact contents that are
staged under different directory names by producer and verifier.

With --classified-files, --dist-dir is an unsigned file-only workspace with
dist/release-files.json. A non-empty --stage-dir is required and must not exist:
admit and verify the privately staged classified files before installing them.
The mode fixes the manifest prefix to '.' and conflicts with --canonical-payload
and an explicit --manifest-root. No source or artifact authorization is implied.

EXAMPLE:
   forge-ci release validate-dist --dist-dir dist --expected-digest sha256:abc...`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "canonical-payload", Usage: "admit and verify the entire dist/ plus premade-sboms/ workspace; fixes manifest prefix to '.'"},
			&cli.BoolFlag{Name: "classified-files", Usage: "admit and privately stage a closed unsigned file-only workspace; requires a fresh --stage-dir; fixes manifest prefix to '.'"},
			&cli.StringFlag{Name: flagDistDir, Value: defaultDistDir, Usage: "directory to verify"},
			&cli.StringFlag{Name: "expected-digest", Sources: cli.EnvVars("EXPECTED_DIGEST", "DIST_DIGEST"), Usage: "expected dist digest (from the build job)"},
			&cli.StringFlag{Name: "digest-file", Usage: "regular producer digest recording; exclusive with --expected-digest"},
			&cli.StringFlag{Name: "stage-dir", Usage: "privately copy, verify staged bytes and install into this new directory before success"},
			&cli.StringFlag{Name: "manifest-root", Usage: "path prefix written into the sha256sum manifest during digest recomputation (defaults to --dist-dir)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := validateDistPayloadMode(cmd); err != nil {
				return err
			}

			if cmd.Bool("classified-files") && cmd.String("stage-dir") == "" {
				return fmt.Errorf("--classified-files requires a fresh --stage-dir: %w", errs.ErrUsage)
			}

			dir := cmd.String(flagDistDir)

			digest := cmd.String("expected-digest")
			if file := cmd.String("digest-file"); file != "" {
				if digest != "" {
					return fmt.Errorf("use --digest-file or --expected-digest, not both: %w", errs.ErrUsage)
				}

				var err error

				digest, err = apprelease.ReadDistDigestFile(file)
				if err != nil {
					return err
				}
			}

			var err error

			destination := cmd.String("stage-dir")
			switch {
			case cmd.Bool("classified-files"):
				err = apprelease.StageVerifiedClassifiedFilePayload(ctx, dir, destination, digest)
			case destination != "" && cmd.Bool("canonical-payload"):
				err = apprelease.StageVerifiedCanonicalPayload(ctx, dir, destination, digest)
			case destination != "":
				err = apprelease.StageVerifiedDist(ctx, dir, destination, digest, cmd.String("manifest-root"))
			case cmd.Bool("canonical-payload"):
				err = apprelease.VerifyCanonicalPayload(dir, digest)
			default:
				err = apprelease.VerifyDistWithManifestRoot(dir, digest, cmd.String("manifest-root"))
			}

			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(os.Stderr, "dist/ integrity verified: %s\n", dir)

			return nil
		},
	}
}

func validateDistPayloadMode(cmd *cli.Command) error {
	if cmd.Bool("classified-files") {
		if cmd.Bool("canonical-payload") {
			return fmt.Errorf("use --classified-files or --canonical-payload, not both: %w", errs.ErrUsage)
		}

		if cmd.IsSet("manifest-root") {
			return fmt.Errorf("--classified-files fixes --manifest-root to '.': %w", errs.ErrUsage)
		}

		if cmd.String(flagDistDir) == "" {
			return fmt.Errorf("--classified-files requires a non-empty --dist-dir: %w", errs.ErrUsage)
		}
	}

	if cmd.Bool("canonical-payload") && cmd.IsSet("manifest-root") {
		return fmt.Errorf("--canonical-payload fixes --manifest-root to '.': %w", errs.ErrUsage)
	}

	return nil
}
