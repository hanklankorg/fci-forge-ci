// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
)

func TestReleaseReadbackAuthenticatesCapturedChecksums(t *testing.T) {
	for _, retain := range []bool{false, true} {
		for _, substitute := range []bool{false, true} {
			t.Run(fmt.Sprintf("retain=%t/substitute=%t", retain, substitute), func(t *testing.T) {
				t.Setenv("FORGE_CI_LIVE_SNAPSHOT_EVIDENCE_DIR", "")
				t.Setenv("FORGE_CI_LIVE_SNAPSHOT_EVIDENCE", "")

				if retain {
					t.Setenv("FORGE_CI_LIVE_SNAPSHOT_EVIDENCE_DIR", t.TempDir())
				}

				names := []string{"app.tar.gz", "checksums.txt", "checksums.txt.bundle", "unused.json"}
				if !substitute {
					names = append(names, "app.tar.gz.bundle")
				}

				evidence, err := NewReleaseEvidence(Target{}, "repo", "v1", "request", names)
				if err != nil {
					t.Fatal(err)
				}

				manifestA := fmt.Appendf(nil, "%x  app.tar.gz\n", sha256.Sum256([]byte("original")))
				manifestB := fmt.Appendf(nil, "%x  app.tar.gz\n", sha256.Sum256([]byte("substitute")))
				calls := map[string]int{}
				download := func(name string) ([]byte, error) {
					calls[name]++

					if name == "checksums.txt" {
						if calls[name] > 1 {
							return manifestB, nil
						}

						return manifestA, nil
					}

					if name == "app.tar.gz" && substitute {
						return []byte("substitute"), nil
					}

					if name == "app.tar.gz" {
						return []byte("original"), nil
					}

					return []byte(name), nil
				}

				verified, err := VerifyReleaseReadback(evidence, names, download, func(body, signature []byte) error {
					if !substitute && string(body) == "original" && string(signature) == "app.tar.gz.bundle" {
						return nil
					}

					if !bytes.Equal(body, manifestA) || string(signature) != "checksums.txt.bundle" {
						return errors.New("inert signature mismatch")
					}

					return nil
				})
				if (err != nil) != substitute || !verified["checksums.txt"] {
					t.Errorf("composed readback: verified=%v err=%v", verified, err)
				}

				for _, name := range names {
					want := 1
					if !retain && name == "unused.json" {
						want = 0
					}

					if calls[name] != want {
						t.Errorf("%s fetched %d times, want %d", name, calls[name], want)
					}
				}

				if retain {
					if !evidence.receipt.Complete {
						t.Fatal("readback lost complete observation evidence")
					}

					retained, err := evidence.readRetained("checksums.txt")
					if err != nil || !bytes.Equal(retained, manifestA) {
						t.Fatalf("retained authenticated document changed: %q, %v", retained, err)
					}
				}
			})
		}
	}
}

func TestReleaseReadbackRetainsFailedVerification(t *testing.T) {
	t.Setenv("FORGE_CI_LIVE_SNAPSHOT_EVIDENCE_DIR", t.TempDir())

	names := []string{"checksums.txt", "checksums.txt.bundle", "payload.tar.gz"}

	evidence, err := NewReleaseEvidence(Target{}, "repo", "v1", "request", names)
	if err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("inert signature failure")

	verified, err := VerifyReleaseReadback(evidence, names, func(name string) ([]byte, error) { return []byte(name), nil }, func([]byte, []byte) error { return wantErr })
	if !errors.Is(err, wantErr) || len(verified) != 0 || !evidence.receipt.Complete {
		t.Fatalf("failed verification lost readback or became trusted: %v, %v, complete=%t", verified, err, evidence.receipt.Complete)
	}
}
