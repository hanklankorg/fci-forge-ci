// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"fmt"
	"strings"
)

// VerifyReleaseBundles pairs offered .bundle assets with their offered subjects.
// The callback owns bounded reads, retention and native trust verification, and
// must fail if either read or verification fails. Only successful subjects enter
// the result; failures are joined so other pairs can still be read and verified.
// Inventory admission and minimum/all-subject signature coverage stay caller-owned.
func VerifyReleaseBundles(names []string, verify func(subject, bundle string) error) (map[string]bool, error) {
	offered := make(map[string]bool, len(names))
	for _, name := range names {
		offered[name] = true
	}

	verified := map[string]bool{}

	var failures []error

	for _, name := range names {
		subject, isBundle := strings.CutSuffix(name, ".bundle")
		if !isBundle {
			continue
		}

		if !offered[subject] {
			failures = append(failures, fmt.Errorf("release bundle %q has no offered subject %q: %w", name, subject, errReleaseEvidence))

			continue
		}

		if err := verify(subject, name); err != nil {
			failures = append(failures, fmt.Errorf("verify release bundle %q for %q: %w", name, subject, err))

			continue
		}

		verified[subject] = true
	}

	return verified, errors.Join(failures...)
}
