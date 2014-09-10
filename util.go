// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// replaceExt replaces the extension of path with the new one.
func replaceExt(path, newExt string) string {
	oldExt := filepath.Ext(path)
	return path[:len(path)-len(oldExt)] + newExt
}

// rmIgnoreGit removes the folder given by the path. The folder itself remains
// as does the .git folder.
func rmIgnoreGit(target string) error {
	var rmPaths []string

	// Walk the structure and find paths to remove.
	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Don't remove the folder itself.
		if path == target {
			return nil
		}

		// Check each component of the filepath to determine if the final path
		// is part of a .git directory.
		for _, c := range strings.Split(path, string(os.PathSeparator)) {
			if c == ".git" {
				return nil
			}
		}

		// Append to slice.
		rmPaths = append(rmPaths, path)
		return nil
	})
	if err != nil {
		return err
	}

	// Remove the paths.
	for _, path := range rmPaths {
		// Remove the path.
		err = os.RemoveAll(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// cp copies all files from the source directory to the destination directory.
// It logs each folder that it copies (but not individual files).
func cp(from, to string) error {
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Open the file.
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}

		// Stat the file.
		fi, err := srcFile.Stat()
		if err != nil {
			return err
		}

		// Determine the destination filepath and create the needed directory
		// structure.
		dst := filepath.Join(to, strings.TrimPrefix(path, absRootDir))
		err = os.MkdirAll(filepath.Dir(dst), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}

		// If it's a directory, print that we are copying it and don't do
		// anything more.
		if fi.Mode().IsDir() {
			log.Printf("cp -r %s %s\n", cleanPath(path), cleanPath(dst))
			return nil
		}

		// If it's not a regular file, don't do anything more.
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Create the destination file.
		dstFile, err := os.Create(dst)
		if err != nil {
			return err
		}

		// Perform the copy.
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// prefixWriter wraps an io.Writer and causes each Write to also write the
// given prefix.
type prefixWriter struct {
	out    io.Writer
	prefix []byte
}

func (p prefixWriter) Write(b []byte) (int, error) {
	_, err := p.out.Write(p.prefix)
	if err != nil {
		return 0, err
	}
	return p.out.Write(b)
}
