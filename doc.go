// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"azul3d.org/semver.v1"
)

var sectionRe = regexp.MustCompile("[^A-Za-z0-9 -]+")

func makeSection(args ...interface{}) map[string]interface{} {
	sectionData := make(map[string]interface{}, 2)

	if len(args) == 4 {
		sectionData["Name"] = args[0].(string)
		sectionData["ID"] = args[1].(string)
		sectionData["HdrClass"] = args[2].(string)
		sectionData["Class"] = args[3].(string)
		return sectionData
	}

	if len(args) >= 1 {
		name := args[0].(string)
		sectionData["Name"] = name

		id := sectionRe.ReplaceAllString(name, "")
		id = strings.Replace(id, " ", "-", -1)
		id = strings.ToLower(id)
		for strings.Contains(id, "--") {
			id = strings.Replace(id, "--", "-", -1)
		}
		id = strings.Trim(id, "-")
		sectionData["ID"] = id
	}

	if len(args) == 2 {
		sectionData["Class"] = args[1].(string)
	}
	if len(args) == 3 {
		sectionData["HdrClass"] = args[1].(string)
		sectionData["Class"] = args[2].(string)
	}
	return sectionData
}

func htmlDoc(s string) template.HTML {
	b := new(bytes.Buffer)
	doc.ToHTML(b, s, nil)
	return template.HTML(b.String())
}

func pkgVersion(path string) (major, minor int) {
	split := strings.Split(path, "v")
	last := split[len(split)-1]
	dots := strings.Split(last, ".")
	major = -1
	minor = -1
	if len(dots) > 0 {
		v, _ := strconv.ParseInt(dots[0], 10, 64)
		major = int(v)
	}
	if len(dots) > 1 {
		v, _ := strconv.ParseInt(dots[1], 10, 64)
		minor = int(v)
	}
	return
}

var errNoPackages = fmt.Errorf("Directory contains no packages.")

func openPkgDoc(path, importPath string) (*doc.Package, *token.FileSet, error) {
	// Parse comments from source files.
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	if len(pkgs) == 0 {
		// No packages in the directory at all.
		return nil, nil, errNoPackages
	}

	// Find the first package with documentation.
	var pkg *doc.Package
	for name := range pkgs {
		pkg = doc.New(pkgs[name], importPath, 0)
		if len(pkg.Doc) > 0 {
			break
		}
	}
	return pkg, fset, nil
}

func genPkgIndex(importables sortedImportables) error {
	// Make copy of slice before deleting elements below.
	cpy := make(sortedImportables, len(importables))
	copy(cpy, importables)
	importables = cpy

	// Map used to serve templates with data.
	tmplData := make(map[string]interface{}, 32)

	// Create map of synopses to importables' indices.
	synopses := make([]string, len(importables))
	for i, imp := range importables {
		mostRecentVersion := imp.Versions[0]
		path := filepath.Join(baseImport, imp.RelPkgPath+"."+mostRecentVersion)

		// Source directory is not relative, but is under $GOPATH/src.
		absPath := filepath.Join(GOPATH, "src", path)

		// Open the package documentation.
		pkg, _, err := openPkgDoc(absPath, path)
		if err != nil {
			log.Println("            -> ERROR", err)
			synopses[i] = "Package synopsis is unavailable."
			continue
		}
		synopses[i] = doc.Synopsis(pkg.Doc)
		if len(synopses[i]) == 0 {
			synopses[i] = "Package synopsis is unavailable."
		}
	}
	tmplData["Synopses"] = synopses

	tmplData["Packages"] = importables
	tmplData["ID"] = func(bad string) string {
		id := strings.Replace(bad, "/", "-", -1)
		return id
	}

	// Create output file in e.g. out/pkgindex.html
	outPath := filepath.Join(*outDir, pkgIndexOut)
	err := os.MkdirAll(filepath.Dir(outPath), os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}

	// Finally, execute the template with the data.
	return tmplRoot.ExecuteTemplate(out, pkgIndexTemplate, tmplData)
}

type importable struct {
	RelPkgPath string
	Versions   []string
}
type sortedImportables []importable

func (s sortedImportables) Len() int      { return len(s) }
func (s sortedImportables) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortedImportables) Less(i, j int) bool {
	return s[i].RelPkgPath < s[j].RelPkgPath
}

// runs "go get -u <path>" to download/update source code.
func gogetu(path string) (err error, stdout, stderr *bytes.Buffer) {
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)

	fmt.Fprintf(stdout, "    go get -u %s\n", path)
	cmd := exec.Command("go", "get", "-u", path)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	//cmd.Stdout = prefixWriter{out: os.Stdout, prefix: []byte("        ")}
	//cmd.Stderr = prefixWriter{out: os.Stderr, prefix: []byte("        ")}
	return cmd.Run(), stdout, stderr
}

// impVersions returns all of the importable versions of the package living at
// the given repo.
func impVersions(repo repo) []string {
	// A map of all versions, used to avoid duplicate e.g. branch/tag versions.
	vmap := make(map[string]bool, len(repo.Tags)+len(repo.Branches))

	// parseVersion attempts to parse the version string (git tag/branch name),
	// if the given version string is a valid version it's major string is
	// returned (dev versions are NOT accepted):
	//
	//  "v1.2" -> semver.Version{Major: 1}
	//  "v2.4.3" -> semver.Version{Major: 2}
	//  "trash string" -> semver.InvalidVersion
	//  "v4-dev" -> semver.InvalidVersion
	//  "v4.2.3-dev" -> semver.InvalidVersion
	//
	parseVersion := func(version string) semver.Version {
		v := semver.ParseVersion(version)
		if v == semver.InvalidVersion || v.Dev {
			return semver.InvalidVersion
		}
		v.Minor = -1
		v.Patch = -1
		return v
	}

	// Load all valid versions from tag/branch names into the map.
	for _, tag := range repo.Tags {
		v := parseVersion(*tag.Name)
		if v != semver.InvalidVersion {
			vmap[v.String()] = true
		}
	}
	for _, branch := range repo.Branches {
		v := parseVersion(*branch.Name)
		if v != semver.InvalidVersion {
			vmap[v.String()] = true
		}
	}

	// Build a sorted list of versions, since we only care about major versions
	// here we can just use string sorting.
	versions := make([]string, 0, len(vmap))
	for v := range vmap {
		versions = append(versions, v)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions
}

func generateDocs() error {
	if *docsFlag == false && *updateFlag == false {
		log.Println("Skipping updates of local repositories (-update=false).")
		log.Println("Skipping generation of package documentation (-docs=false).")
		return nil
	}

	log.Println("Scanning github repositories...")
	repos, err := fetchRepos()
	if err != nil {
		return err
	}

	// Remove ignored repos from the map.
	for repoName := range repos {
		_, ignored := ignoredRepos[repoName]
		if ignored {
			delete(repos, repoName)
			continue
		} else {
			log.Printf(" - %s\n", repoName)
		}
	}

	if *updateFlag == false {
		log.Println("Skipping updates of local repositories (-update=false).")
	} else {
		log.Println("Updating local repositories...")
		for repoName, repo := range repos {
			for _, tag := range repo.Tags {
				path := importURL(repoName, *tag.Name)
				err, stdout, stderr := gogetu(path)
				stdout.WriteTo(os.Stdout)
				stderr.WriteTo(os.Stderr)
				if err != nil {
					log.Println("        -> ERROR", err)
				}
			}
		}
	}

	if *docsFlag == false {
		log.Println("Skipping generation of package documentation (-docs=false).")
	} else {
		log.Println("Generating package documentation...")

		// Create a list of package paths and importable versions.
		var importables sortedImportables
		for repoName, repo := range repos {
			versions := impVersions(repo)
			if len(versions) == 0 {
				continue
			}
			importables = append(importables, importable{
				RelPkgPath: dashToSlash(repoName),
				Versions:   impVersions(repo),
			})
		}
		sort.Sort(importables)

		// Package index.
		log.Println("    genPkgIndex")
		err := genPkgIndex(importables)
		if err != nil {
			log.Println("        -> ERROR", err)
		}
	}

	return nil
}
