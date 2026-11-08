// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type developmentImageEvidence struct {
	top, child, revision     string
	parent, manifest, config []byte
}

func developmentImageJSON(t *testing.T, value any) []byte {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

func developmentImageHash(t *testing.T, raw []byte) v1.Hash {
	t.Helper()

	hash, _, err := v1.SHA256(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	return hash
}

func developmentImageTestEvidence(t *testing.T, os, arch, revision string, index bool) developmentImageEvidence {
	t.Helper()

	config := developmentImageJSON(t, v1.ConfigFile{OS: os, Architecture: arch,
		Config: v1.Config{Labels: map[string]string{"org.opencontainers.image.revision": revision}}})
	manifest := developmentImageJSON(t, v1.Manifest{SchemaVersion: 2, MediaType: types.OCIManifestSchema1,
		Config: v1.Descriptor{MediaType: types.OCIConfigJSON, Digest: developmentImageHash(t, config), Size: int64(len(config))}})
	child := developmentImageHash(t, manifest)
	parent := manifest

	if index {
		parent = developmentImageJSON(t, v1.IndexManifest{SchemaVersion: 2, MediaType: types.OCIImageIndex,
			Manifests: []v1.Descriptor{{MediaType: types.OCIManifestSchema1, Digest: child, Size: int64(len(manifest)),
				Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}}})
	}

	return developmentImageEvidence{top: developmentImageHash(t, parent).String(), child: child.String(), revision: revision,
		parent: parent, manifest: manifest, config: config}
}

func (e developmentImageEvidence) validate() error {
	return validateDevelopmentImage(e.top, e.child, e.revision, e.parent, e.manifest, e.config)
}

func TestDevelopmentImageIdentity(t *testing.T) {
	for _, index := range []bool{false, true} {
		e := developmentImageTestEvidence(t, "linux", "amd64", strings.Repeat("a", 40), index)
		if err := e.validate(); err != nil {
			t.Fatal(err)
		}

		if (e.top == e.child) == index {
			t.Fatal("index and child semantic identities were conflated")
		}
	}
}

func TestDevelopmentImageIdentityRefusesUnboundEvidence(t *testing.T) {
	const revision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	for _, test := range []struct {
		name   string
		change func(*developmentImageEvidence)
	}{
		{"wrong top-level digest", func(e *developmentImageEvidence) { e.top = "sha256:" + strings.Repeat("b", 64) }},
		{"top-level replaced with child", func(e *developmentImageEvidence) { e.top = e.child }},
		{"tampered parent bytes", func(e *developmentImageEvidence) { e.parent = append(bytes.Clone(e.parent), ' ') }},
		{"unrelated valid child", func(e *developmentImageEvidence) {
			other := developmentImageTestEvidence(t, "linux", "amd64", strings.Repeat("b", 40), false)
			e.child, e.manifest, e.config = other.child, other.manifest, other.config
		}},
		{"malformed child stdout", func(e *developmentImageEvidence) { e.child += "\nextra" }},
		{"tampered child payload", func(e *developmentImageEvidence) { e.manifest = append(bytes.Clone(e.manifest), ' ') }},
		{"tampered config payload", func(e *developmentImageEvidence) { e.config = append(bytes.Clone(e.config), ' ') }},
		{"wrong config architecture", func(e *developmentImageEvidence) {
			*e = developmentImageTestEvidence(t, "linux", "arm64", revision, true)
		}},
		{"wrong config OS", func(e *developmentImageEvidence) {
			*e = developmentImageTestEvidence(t, "windows", "amd64", revision, true)
		}},
		{"wrong source revision", func(e *developmentImageEvidence) { e.revision = strings.Repeat("b", 40) }},
		{"missing source revision", func(e *developmentImageEvidence) { e.revision = "" }},
		{"single manifest wrong child", func(e *developmentImageEvidence) {
			*e = developmentImageTestEvidence(t, "linux", "amd64", revision, false)
			e.child = "sha256:" + strings.Repeat("b", 64)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := developmentImageTestEvidence(t, "linux", "amd64", revision, true)
			test.change(&e)

			if err := e.validate(); err == nil {
				t.Fatal("unbound evidence accepted")
			}
		})
	}
}

func TestDevelopmentImageIdentityRefusesIndexMembershipChanges(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*v1.IndexManifest)
	}{
		{"no child", func(m *v1.IndexManifest) { m.Manifests = nil }},
		{"unrelated child", func(m *v1.IndexManifest) { m.Manifests[0].Digest.Hex = strings.Repeat("b", 64) }},
		{"wrong descriptor architecture", func(m *v1.IndexManifest) { m.Manifests[0].Platform.Architecture = "arm64" }},
		{"wrong descriptor OS", func(m *v1.IndexManifest) { m.Manifests[0].Platform.OS = "windows" }},
		{"missing platform", func(m *v1.IndexManifest) { m.Manifests[0].Platform = nil }},
		{"duplicate child", func(m *v1.IndexManifest) { m.Manifests = append(m.Manifests, m.Manifests[0]) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := developmentImageTestEvidence(t, "linux", "amd64", strings.Repeat("a", 40), true)

			var index v1.IndexManifest
			if err := json.Unmarshal(e.parent, &index); err != nil {
				t.Fatal(err)
			}

			test.change(&index)
			e.parent = developmentImageJSON(t, index)
			e.top = developmentImageHash(t, e.parent).String()

			if err := e.validate(); err == nil {
				t.Fatal("changed index membership accepted even with a matching parent hash")
			}
		})
	}
}

func TestDevelopmentImageReceiptKeepsPublishedTopLevelIdentity(t *testing.T) {
	const image = "registry.invalid/owner/image"

	evidence := developmentImageTestEvidence(t, "linux", "amd64", strings.Repeat("a", 40), true)
	tree := map[string][]byte{
		"commit.txt": []byte(evidence.revision + "\n"), "image-digest.txt": []byte(evidence.top + "\n"),
		"image-ref.txt": []byte(image + "@" + evidence.top + "\n"), "image-tag.txt": []byte(image + ":dev-" + evidence.revision + "\n"),
	}

	if err := ValidateDevelopmentImageReceipt(tree, image, evidence.revision, evidence.top); err != nil {
		t.Fatal(err)
	}

	for path := range tree {
		changed := maps.Clone(tree)
		changed[path] = []byte("wrong\n")

		if err := ValidateDevelopmentImageReceipt(changed, image, evidence.revision, evidence.top); err == nil {
			t.Fatalf("wrong receipt %s accepted", path)
		}

		delete(changed, path)

		if err := ValidateDevelopmentImageReceipt(changed, image, evidence.revision, evidence.top); err == nil {
			t.Fatalf("missing receipt %s accepted", path)
		}
	}

	changed := maps.Clone(tree)
	changed["image-digest.txt"] = []byte(evidence.child + "\n")

	if err := ValidateDevelopmentImageReceipt(changed, image, evidence.revision, evidence.top); err == nil {
		t.Fatal("child digest substituted for published receipt digest")
	}

	changed = maps.Clone(tree)
	changed["extra"] = nil

	if err := ValidateDevelopmentImageReceipt(changed, image, evidence.revision, evidence.top); err == nil {
		t.Fatal("extra receipt file accepted")
	}
}
