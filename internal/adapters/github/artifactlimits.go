// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package github

import (
	"context"
	"fmt"
	"io"

	gogithub "github.com/google/go-github/v76/github"

	domainartifact "github.com/diggsweden/forge-ci/internal/domain/artifact"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/provider"
)

type artifactArchiveLimits struct {
	compressed, expanded, declared int64
	entries                        int
	bounded                        bool
}

func exactArchiveLimits(in provider.RunArtifactDownload, artifact *gogithub.Artifact) artifactArchiveLimits {
	limits := artifactArchiveLimits{compressed: domainartifact.MaxArchiveBytes, expanded: domainartifact.MaxTotalBytes, entries: domainartifact.MaxFileCount, declared: -1}
	if in.Limits != nil {
		limits.compressed = in.Limits.MaxArchiveBytes
		limits.expanded = in.Limits.MaxExpandedBytes
		limits.entries = in.Limits.MaxEntries
		limits.declared = artifact.GetSizeInBytes()
		limits.bounded = true
	}

	return limits
}

// Reuse the native copy primitive, but keep its sentinel byte off disk for
// opt-in hard budgets. Legacy callers retain their existing copy behavior.
func copyArtifactBytes(ctx context.Context, dst io.Writer, src io.Reader, limit int64, bounded bool) (int64, error) {
	if bounded {
		dst = &artifactLimitWriter{check: ctx.Err, dst: dst, remaining: limit}
	}

	return domainartifact.CopyAtMost(dst, src, limit)
}

type artifactLimitWriter struct {
	check     func() error
	dst       io.Writer
	remaining int64
}

func (writer *artifactLimitWriter) Write(body []byte) (int, error) {
	if err := writer.check(); err != nil {
		return 0, err
	}

	allowed := min(int64(len(body)), writer.remaining)
	written, err := writer.dst.Write(body[:allowed])
	writer.remaining -= int64(written)

	if err == nil && int64(len(body)) > allowed {
		err = fmt.Errorf("artifact stream exceeds hard byte budget: %w", errs.ErrValidation)
	}

	return written, err
}
