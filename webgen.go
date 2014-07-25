// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/goauth2/oauth"
	"flag"
	"github.com/google/go-github/github"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TODO:
//  - Types in package documentation should link to eachother.
//  - Source files in package documentation should link to GitHub view.

const (
	contentDirName   = "content"
	pagesDirName     = "pages"
	templatesDirName = "templates"
	rootDir          = "src/azul3d.org/cmd/webgen.v0"
	defaultOutDir    = "src/github.com/azul3d/azul3d.github.io"
	pkgDocTemplate   = "pkgdoc.tmpl"
	pkgDocOutDir     = ""
	pkgIndexTemplate = "pkgindex.tmpl"
	pkgIndexOut      = "/packages.html"
	importDomain     = "azul3d.org"
	githubOrg        = "azul3d"
)

func cp(from, to string) error {
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		dst := filepath.Join(to, strings.TrimPrefix(path, absRootDir))
		log.Printf("cp %s %s\n", cleanPath(path), cleanPath(dst))

		err = os.MkdirAll(filepath.Dir(dst), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}

		// Not a file? Don't do anything.
		fi, err := srcFile.Stat()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}

		dstFile, err := os.Create(dst)
		if err != nil {
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func cleanPath(s string) string {
	s = strings.Replace(s, absRootDir, "$WORK", 1)
	s = strings.Replace(s, *outDir, "$OUT", 1)
	return s
}

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

// runs "go get -u <path>" to download/update source code.
func gogetu(path string) error {
	log.Printf("    go get -u %s", path)
	cmd := exec.Command("go", "get", "-u", path)
	cmd.Stdout = prefixWriter{out: os.Stdout, prefix: []byte("        ")}
	cmd.Stderr = prefixWriter{out: os.Stderr, prefix: []byte("        ")}
	return cmd.Run()
}

var (
	GOPATH           = os.Getenv("GOPATH")
	absRootDir       = filepath.Join(GOPATH, rootDir)
	absDefaultOutDir = filepath.Join(GOPATH, defaultOutDir)
	outDir           = flag.String("out", absDefaultOutDir, "output directory to place generated files")
	cleanOutDir      = flag.Bool("clean", true, "clean output directory (deletes all files)")
	httpAddr         = flag.String("http", "", "port to serve files over HTTP after generation, e.g. :8080")
	updateFlag       = flag.Bool("update", true, "update scanned repositories using go get -u")
	docsFlag         = flag.Bool("docs", true, "generate package documentation (links broken otherwise)")
	auth             = flag.Bool("auth", true, "authenticate with GitHub using $GITHUB_API_TOKEN")
	pushAfter        = flag.Bool("push", true, "run git add, commit, and push in the output directory after generation")

	tmplRoot *template.Template
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	if len(API_TOKEN) == 0 {
		if *auth {
			log.Println("$GITHUB_API_TOKEN not set to a GitHub API token!")
			log.Fatal("Continue without authentication using -auth=false")
		}
		ghClient = github.NewClient(nil)
	} else {
		t := &oauth.Transport{
			Token: &oauth.Token{AccessToken: API_TOKEN},
		}
		ghClient = github.NewClient(t.Client())
	}

	if len(GOPATH) == 0 {
		log.Fatal("GOPATH is invalid.")
	}

	log.Println("WORK =", strings.Replace(absRootDir, GOPATH, "$GOPATH", -1))
	log.Println("OUT =", strings.Replace(*outDir, GOPATH, "$GOPATH", -1))

	if *cleanOutDir {
		log.Println("rm -rf", cleanPath(*outDir))
		err := filepath.Walk(*outDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path != *outDir && !strings.Contains(path, ".git") {
				return os.RemoveAll(path)
			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	// Copy content folder.
	content := filepath.Join(absRootDir, contentDirName)
	err := cp(content, *outDir)
	if err != nil {
		log.Fatal(err)
	}

	tmplRoot, err = template.New("root").Funcs(map[string]interface{}{
		"section":      makeSection,
		"filepathJoin": filepath.Join,
	}).ParseGlob(filepath.Join(absRootDir, templatesDirName, "*.tmpl"))
	if err != nil {
		log.Fatal(err)
	}

	// Execute each page template as needed.
	pagesDir := filepath.Join(absRootDir, pagesDirName)
	err = filepath.Walk(pagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Open template file (or folder).
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// If not a template file, don't do anything.
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Grab the relative directory path.
		dir := strings.TrimPrefix(path, absRootDir)
		dir = strings.TrimPrefix(dir, string(os.PathSeparator))
		dir = filepath.Dir(dir)

		// Grab template name.
		name := strings.TrimSuffix(filepath.Base(path), ".tmpl")

		// Create output directory structure.
		dirNoPages := strings.TrimPrefix(dir, pagesDirName)
		htmlOut := filepath.Join(*outDir, dirNoPages, name+".html")
		htmlOutDir := filepath.Dir(htmlOut)
		log.Println("mkdir", cleanPath(htmlOutDir))
		err = os.MkdirAll(htmlOutDir, os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}

		// Read file data.
		fdata, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		// Create and load template.
		tmplName := filepath.Join(dirNoPages, name)
		tmpl, err := tmplRoot.New(tmplName).Parse(string(fdata))
		if err != nil {
			return err
		}

		// Create template output HTML file.
		of, err := os.Create(htmlOut)
		if err != nil {
			return err
		}

		// Execute template.
		log.Println("execute", tmplName, ">", cleanPath(htmlOut))
		err = tmpl.ExecuteTemplate(of, tmplName, nil)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	err = generateDocs()
	if err != nil {
		log.Fatal(err)
	}

	if *pushAfter {
		log.Println("Pushing changes to remote...")
		log.Println("    Repo Root:", *outDir)
		err := gitadda(*outDir)
		if err != nil {
			log.Println("    ", err)
		}
		err = gitcommitam(*outDir, "Automatic commit by webgen command line tool.")
		if err != nil {
			log.Println("    ", err)
		}
		err = gitpush(*outDir)
		if err != nil {
			log.Println("    ", err)
		}
	}

	if len(*httpAddr) > 0 {
		log.Println("Listening on HTTP", *httpAddr)
		http.Handle("/", http.FileServer(http.Dir(*outDir)))
		//http.Handle("/tmpfiles/", http.StripPrefix("/tmpfiles/", http.FileServer(http.Dir("/tmp"))))
		//fs := http.FileServer(http.Dir(*outDir))
		//http.Handle("/", http.StripPrefix(*outDir, fs))
		err := http.ListenAndServe(*httpAddr, nil)
		if err != nil {
			log.Fatal(err)
		}
	}
}
