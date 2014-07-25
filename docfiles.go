// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/doc"
	"path/filepath"
	"sort"
	"strings"
)

// Code for package documentation in the "Files" section.

type sourceFile struct {
	Name    string
	ViewURL string
}
type sortedSourceFiles []sourceFile

func (s sortedSourceFiles) Len() int           { return len(s) }
func (s sortedSourceFiles) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedSourceFiles) Less(i, j int) bool { return s[i].Name < s[j].Name }

type buildTagSource struct {
	Title   string
	Sources []sourceFile
}
type sortedBuildTagSources []*buildTagSource

func (s sortedBuildTagSources) Len() int           { return len(s) }
func (s sortedBuildTagSources) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedBuildTagSources) Less(i, j int) bool { return s[i].Title < s[j].Title }

func collectSources(absPackageRoot, sourceViewURL string, pkg *doc.Package) (generic []sourceFile, tagged []*buildTagSource, err error) {
	tags := make(map[string]*buildTagSource)
	for _, path := range pkg.Filenames {
		rel, err := filepath.Rel(absPackageRoot, path)
		if err != nil {
			return nil, nil, err
		}
		name := filepath.Base(path)
		src := sourceFile{
			Name:    name,
			ViewURL: filepath.Join(sourceViewURL, rel),
		}
		split := strings.Split(name, "_")
		if len(split) <= 1 {
			// No tags.
			generic = append(generic, src)
			continue
		}
		tagName := strings.Join(split[1:], " ")
		tag, ok := tags[tagName]
		if !ok {
			title := strings.TrimSuffix(tagName, ".go")
			title = strings.Title(title)
			tag = &buildTagSource{Title: title}
			tags[tagName] = tag
		}
		tag.Sources = append(tag.Sources, src)
	}
	for _, tag := range tags {
		tagged = append(tagged, tag)
	}

	// Sort lists of sourceFile by string.
	sort.Sort(sortedSourceFiles(generic))
	for _, t := range tagged {
		sort.Sort(sortedSourceFiles(t.Sources))
	}

	// Sort list of tagged sources by string.
	sort.Sort(sortedBuildTagSources(tagged))

	return
}
