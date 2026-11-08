// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package snapshotimage

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/pathsafe"
)

var errFileOutsideDestination = errors.New("snapshot inventory file lies outside its dependency destination")

// layNPMPackage places a build's verified npm package at its context root.
//
// A Containerfile copies the package's files by their package paths,
// `COPY dist/ /app/`, because that is where every release image job lays the
// unpacked package: GitHub's extract-npm-tarball, and the Forgejo and GitLab
// release builds' image context laid over the checkout. The snapshot verifies
// the package under <context>/.forge-ci/snapshot-inputs/<name>, the directory
// the inventory owns, and this step copies the verified files to the context
// root with their recorded bytes and modes, through the same confined staging
// restore-context uses: symlinked or non-regular targets refuse before any
// write, and an interrupted install rolls back.
func layNPMPackage(build snapshotimage.Build, inventory snapshotimage.DependencyInventory, inventoryFile string) error {
	files, sources, err := npmContextFiles(build, inventory, inventoryFile)
	if err != nil || len(files) == 0 {
		return err
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	checkout, err := pathsafe.OpenRoot(".")
	if err != nil {
		return err
	}

	defer func() { _ = checkout.Close() }()

	for _, path := range paths {
		if err := preflightRestoreTarget(checkout, path); err != nil {
			return err
		}
	}

	stage, err := pathsafe.NewArtifactStaging(".")
	if err != nil {
		return err
	}

	defer func() { _ = stage.Close() }()

	for _, path := range paths {
		if err := stageContextFile(sources[path], stage.Root(), files[path]); err != nil {
			return err
		}
	}

	return stage.Install()
}

// npmContextFiles maps each verified npm file of the build to its place at
// the context root, keyed by that target path, with the verified source path
// beside it.
func npmContextFiles(build snapshotimage.Build, inventory snapshotimage.DependencyInventory, inventoryFile string) (map[string]snapshotimage.DependencyFile, map[string]string, error) {
	files := map[string]snapshotimage.DependencyFile{}
	sources := map[string]string{}

	for _, record := range inventory.Dependencies {
		if record.ImageName != build.Name || record.Platform != "linux/"+build.Arch || record.ProjectType != projecttype.NPM {
			continue
		}

		for _, file := range record.Files {
			rel, err := filepath.Rel(record.Destination, file.Path)
			if err != nil || !filepath.IsLocal(rel) {
				return nil, nil, fmt.Errorf("%w: %s", errFileOutsideDestination, file.Path)
			}

			target := filepath.ToSlash(filepath.Join(build.Context, rel))
			if err := restorePath(target, inventoryFile); err != nil {
				return nil, nil, err
			}

			placed := file
			placed.Path = target
			files[target] = placed
			sources[target] = file.Path
		}
	}

	return files, sources, nil
}
