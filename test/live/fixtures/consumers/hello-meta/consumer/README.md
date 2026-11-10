# Hello meta

A shell installer, a script library and a hidden configuration file,
distributed through Git: run `sh install.sh`. There is nothing to compile, no
container, no version file and no generated SBOM.

Its workflows name Forge CI where it is published, at a placeholder commit;
the live tier puts the Forge CI under test in its place.

The release declares `project-type: meta`, renders its notes with `git-cliff`
and sets `changelog.skipversionbump: true`. It runs no build and no version
bump, so main and the signed final tag stay at the requested commit, with
every file as it was. Nothing of the checkout is attached to the release: its
only files are the checksum list and that list's signature.

The pull-request gate lints, and the consumer's own job runs the installer.
