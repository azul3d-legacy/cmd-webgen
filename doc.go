// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"github.com/google/go-github/github"
	"go/doc"
	"go/parser"
	"go/token"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

func genPkgDoc(relPkgPath, thisVersionTag string, versions []string) (error, string) {
	path := filepath.Join(baseImport, relPkgPath+"."+thisVersionTag)

	// Source directory is not relative, but is under $GOPATH/src.
	absPath := filepath.Join(GOPATH, "src", path)

	// Open the package documentation.
	pkg, fset, err := openPkgDoc(absPath, path)
	if err != nil {
		return err, ""
	}

	// Store pkg.Doc before filtering (filter empties the string).
	pkgDoc := pkg.Doc

	// Map used to serve templates with data.
	tmplData := make(map[string]interface{}, 32)

	// Filter out any test, benchmark, or example code.
	pkg.Filter(func(name string) bool {
		hasAnyPrefix := func(prefixes ...string) bool {
			for _, p := range prefixes {
				if strings.HasPrefix(name, p) {
					return true
				}
			}
			return false
		}
		return !hasAnyPrefix("Test", "Benchmark", "Example")
	})

	// Store some generic package information.
	tmplData["Pkg"] = pkg
	major, minor := pkgVersion(path)
	tmplData["BaseImport"] = baseImport
	tmplData["RelPkgPath"] = relPkgPath
	tmplData["VersionTag"] = thisVersionTag
	tmplData["Major"] = major
	tmplData["Minor"] = minor
	//tmplData["Synopsis"] = doc.Synopsis(pkgDoc)
	tmplData["Versions"] = versions

	// Collect index entries.
	tmplData["IndexFuncs"] = collectIndexFuncs(pkg, fset)
	tmplData["IndexTypes"] = collectIndexTypes(pkg, fset)

	// Collect constants and variables.
	vars, consts := collectValues(pkg, fset)
	tmplData["Vars"] = vars
	tmplData["Consts"] = consts

	// Generate HTML package comments.
	tmplData["HTMLDoc"] = htmlDoc(pkgDoc)

	// Collect source files.
	absPackageRoot := filepath.Join(GOPATH, "src", path)
	sourceURL := filepath.Join("https://github.com", githubOrg, relPkgPath, "blob", thisVersionTag)
	generic, tagged, err := collectSources(absPackageRoot, sourceURL, pkg)
	if err != nil {
		return err, ""
	}
	tmplData["GenericSources"] = generic
	tmplData["TaggedSources"] = tagged

	// path is something like azul3d.org/pkgname.v1, we just want the
	// pkgname.v1 part.
	postDomainPath, err := filepath.Rel(importDomain, path)
	if err != nil {
		return err, ""
	}

	// Create output file in e.g. out/pkgname.v1
	outPath := filepath.Join(*outDir, pkgDocOutDir, postDomainPath)
	err = os.MkdirAll(filepath.Dir(outPath), os.ModeDir|os.ModePerm)
	if err != nil {
		return err, ""
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err, ""
	}

	// Finally, execute the template with the data.
	return tmplRoot.ExecuteTemplate(out, pkgDocTemplate, tmplData), outPath
}

func genPkgIndex(importables sortedImportables) (error, string) {
	searching := true
search:
	for searching {
		for i, imp := range importables {
			if len(imp.VersionTags) == 1 && imp.VersionTags[0] == "v0" {
				// Only has version zero tag (i.e. package not released yet).
				// Do not index it on the packages page.
				importables = append(importables[:i], importables[i+1:]...)
				continue search
			}
		}
		searching = false
	}

	// Map used to serve templates with data.
	tmplData := make(map[string]interface{}, 32)

	// Create map of synopses to importables' indices.
	synopses := make([]string, len(importables))
	for i, imp := range importables {
		mostRecentVersion := imp.VersionTags[0]
		path := filepath.Join(baseImport, imp.RelPkgPath+"."+mostRecentVersion)

		// Source directory is not relative, but is under $GOPATH/src.
		absPath := filepath.Join(GOPATH, "src", path)

		// Open the package documentation.
		pkg, _, err := openPkgDoc(absPath, path)
		if err != nil {
			log.Println("            -> ERROR", err)
			synopses[i] = "Synopsis is unavailable."
			continue
		}
		synopses[i] = doc.Synopsis(pkg.Doc)
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
		return err, ""
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err, ""
	}

	// Finally, execute the template with the data.
	return tmplRoot.ExecuteTemplate(out, pkgIndexTemplate, tmplData), outPath
}

type importable struct {
	RelPkgPath  string
	VersionTags []string
}
type sortedImportables []importable

func (s sortedImportables) Len() int      { return len(s) }
func (s sortedImportables) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortedImportables) Less(i, j int) bool {
	return s[i].RelPkgPath < s[j].RelPkgPath
}

func generateDocs() error {
	log.Println("Scanning github repositories...")
	repos, err := fetchRepos(ghClient)
	if err != nil {
		return err
	}

	// Remove ignored repos from the map.
	for repoName, repo := range repos {
		_, ignored := ignoredRepos[repoName]
		if ignored {
			log.Printf("    ignoring %q - %s\n", repoName, *repo.URL)
			delete(repos, repoName)
			continue
		} else {
			log.Printf("    found repo %q - %s\n", repoName, *repo.URL)
		}

		// Add implicit version zero (development version / tip) tag to the
		// slice.
		v0 := "v0"
		repo.Tags = append(repo.Tags, github.RepositoryTag{
			Name: &v0,
		})
		repos[repoName] = repo

		// List packages found in each repo.
		for _, tag := range repo.Tags {
			log.Println("        Package -", importURL(repoName, *tag.Name))
		}
	}
	log.Println("    done.")

	if *updateFlag == false {
		log.Println("Skipping updates of local repositories (-update=false).")
	} else {
		log.Println("Updating local repositories...")
		for repoName, repo := range repos {
			for _, tag := range repo.Tags {
				path := importURL(repoName, *tag.Name)
				err := gogetu(path)
				if err != nil {
					log.Println("        -> ERROR", err)
				}
			}
		}
		log.Println("    done.")
	}

	if *docsFlag == false {
		log.Println("Skipping generation of package documentation (-docs=false).")
	} else {
		log.Println("Generating package documentation...")

		// Create a list of base import paths and all versions of the package.
		var importables sortedImportables
		for repoName, repo := range repos {
			versionTags := make([]string, 0, len(repo.Tags))
			for _, tag := range repo.Tags {
				versionTags = append(versionTags, *tag.Name)
			}
			sort.Sort(sort.Reverse(sort.StringSlice(versionTags)))
			importables = append(importables, importable{
				RelPkgPath:  dashToSlash(repoName),
				VersionTags: versionTags,
			})
		}
		sort.Sort(importables)

		// Package index.
		log.Println("    genPkgIndex")
		err, outPath := genPkgIndex(importables)
		if err != nil {
			log.Println("        -> ERROR", err)
		} else {
			log.Println("        ->", outPath)
		}

		for _, imp := range importables {
			for _, versionTag := range imp.VersionTags {
				log.Printf("    genPkgDoc - %s\n", filepath.Join(baseImport, imp.RelPkgPath+"."+versionTag))
				err, outPath := genPkgDoc(imp.RelPkgPath, versionTag, imp.VersionTags)
				if err != nil {
					log.Println("        -> ERROR", err)
				} else {
					log.Println("        ->", outPath)
				}
			}
		}
		log.Println("    done.")
	}

	return nil
}
