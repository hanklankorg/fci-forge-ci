// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"bytes"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// VerifyDevelopmentImage independently binds the published top-level digest to
// its raw registry payload and the CLI-selected linux/amd64 child. All reads use
// the existing native registry transport/auth/CA scope, not a second client.
func VerifyDevelopmentImage(tb TB, target Target, image, top, child, revision string) error {
	tb.Helper()
	requireAccepted(tb, target)

	opts, err := registryRemoteOptions(target)
	if err != nil {
		return err
	}

	opts = append(opts, remote.WithContext(Context(tb)))
	read := func(digest string) (*remote.Descriptor, error) {
		ref, err := name.NewDigest(image + "@" + digest)
		if err != nil {
			return nil, err
		}

		return remote.Get(ref, opts...)
	}

	parent, err := read(top)
	if err != nil {
		return err
	}

	selected, err := read(child)
	if err != nil {
		return err
	}

	img, err := selected.Image()
	if err != nil {
		return err
	}

	config, err := img.RawConfigFile()
	if err != nil {
		return err
	}

	return validateDevelopmentImage(top, child, revision, parent.Manifest, selected.Manifest, config)
}

func validateDevelopmentImage(top, child, revision string, parentRaw, childRaw, configRaw []byte) error {
	if !container.ValidDigest(top) || !container.ValidDigest(child) || !fixtureObjectID(revision) {
		return fmt.Errorf("development image requires immutable digests and source revision: %w", errs.ErrValidation)
	}

	if err := developmentPayloadDigest(parentRaw, top); err != nil {
		return fmt.Errorf("published manifest payload: %w", err)
	}

	if err := container.ValidateImageIndex(parentRaw, []container.PlatformDigest{{Platform: "linux/amd64", Digest: child}}); err != nil {
		return fmt.Errorf("published manifest child membership: %w", err)
	}

	if err := developmentPayloadDigest(childRaw, child); err != nil {
		return fmt.Errorf("selected child payload: %w", err)
	}

	manifest, err := v1.ParseManifest(bytes.NewReader(childRaw))
	if err != nil {
		return err
	}

	if manifest.SchemaVersion != 2 || !manifest.MediaType.IsImage() {
		return fmt.Errorf("selected child is not an image manifest: %w", errs.ErrValidation)
	}

	return validateDevelopmentImageConfig(manifest.Config, configRaw, revision)
}

func validateDevelopmentImageConfig(descriptor v1.Descriptor, configRaw []byte, revision string) error {
	if err := developmentPayloadDigest(configRaw, descriptor.Digest.String()); err != nil {
		return fmt.Errorf("selected child config payload: %w", err)
	}

	if int64(len(configRaw)) != descriptor.Size {
		return fmt.Errorf("selected child config size differs: %w", errs.ErrValidation)
	}

	config, err := v1.ParseConfigFile(bytes.NewReader(configRaw))
	if err != nil {
		return err
	}

	if config.OS != "linux" || config.Architecture != "amd64" || config.Config.Labels["org.opencontainers.image.revision"] != revision {
		return fmt.Errorf("selected child platform or revision differs: %w", errs.ErrValidation)
	}

	return nil
}

func developmentPayloadDigest(raw []byte, want string) error {
	digest, _, err := v1.SHA256(bytes.NewReader(raw))
	if err != nil {
		return err
	}

	if digest.String() != want {
		return fmt.Errorf("manifest bytes do not match expected digest %s: %w", want, errs.ErrValidation)
	}

	return nil
}

// ValidateDevelopmentImageReceipt keeps all four producer receipt values bound
// to the published top-level identity, never to a platform-selected child.
func ValidateDevelopmentImageReceipt(tree map[string][]byte, image, revision, top string) error {
	if !container.ValidDigest(top) || !fixtureObjectID(revision) {
		return fmt.Errorf("development receipt identity is invalid: %w", errs.ErrValidation)
	}

	want := map[string]string{
		"commit.txt": revision, "image-digest.txt": top,
		"image-ref.txt": image + "@" + top, "image-tag.txt": image + ":dev-" + revision,
	}
	if len(tree) != len(want) {
		return fmt.Errorf("development receipt requires exactly four files: %w", errs.ErrValidation)
	}

	for path, value := range want {
		if string(tree[path]) != value+"\n" {
			return fmt.Errorf("development receipt %s differs from published source/image identity: %w", path, errs.ErrValidation)
		}
	}

	return nil
}
