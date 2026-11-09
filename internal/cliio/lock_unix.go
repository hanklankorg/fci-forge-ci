// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build unix

package cliio

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// lockExclusive takes a blocking exclusive advisory lock (flock LOCK_EX)
// on f, blocking until no other process holds it.
func lockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlock releases the advisory lock held on f.
func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func singleLink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)

	return ok && stat.Nlink == 1
}

// Nonblocking open prevents a named-pipe replacement from hanging before Stat.
func openReadFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // operator-selected input; the caller validates this descriptor and bounds the read.
}

// openReadFileNoFollow opens path as openReadFile does, but refuses a final
// symbolic link: the open fails with ELOOP instead of following it.
func openReadFileNoFollow(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0) //nolint:gosec // operator-selected input; the caller validates this descriptor and bounds the read.
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%q is a symbolic link, not a regular input file: %w", path, errs.ErrMissingInput)
	}

	return file, err
}

func openRootReadFile(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
