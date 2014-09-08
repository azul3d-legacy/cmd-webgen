// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"html/template"
	"path/filepath"
	"io/ioutil"
	"bytes"
	"strings"
	"log"
	"os"

	bf "github.com/russross/blackfriday"
)

func MarkdownNews(input []byte) []byte {
	// set up the HTML renderer
	htmlFlags := 0
	htmlFlags |= bf.HTML_USE_XHTML
	htmlFlags |= bf.HTML_USE_SMARTYPANTS
	htmlFlags |= bf.HTML_SMARTYPANTS_FRACTIONS
	htmlFlags |= bf.HTML_SMARTYPANTS_LATEX_DASHES
	//htmlFlags |= HTML_SANITIZE_OUTPUT
	renderer := bf.HtmlRenderer(htmlFlags, "", "")

	// set up the parser
	extensions := 0
	extensions |= bf.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= bf.EXTENSION_TABLES
	extensions |= bf.EXTENSION_FENCED_CODE
	extensions |= bf.EXTENSION_AUTOLINK
	extensions |= bf.EXTENSION_STRIKETHROUGH
	extensions |= bf.EXTENSION_SPACE_HEADERS
	extensions |= bf.EXTENSION_HEADER_IDS

	return bf.Markdown(input, renderer, extensions)
}

// findNewsTitle finds the news title in the markdown input. It is expected to
// exist on the first line:
//  # Some Title Here
// The function then returns a string:
//  "Some Title Here"
func findNewsTitle(buf []byte) string {
	lineEnd := bytes.IndexAny(buf, "\n")
	if lineEnd == -1 {
		return ""
	}
	l := string(buf[:lineEnd])
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "#")
	return strings.TrimSpace(l)
}

func generateNews() error {
	log.Println("Generating news articles...")

	// Generate each markdown page as needed.
	newsDir := filepath.Join(absRootDir, newsDirName)
	err := filepath.Walk(newsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Open markdown file (or folder).
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// If not a file, don't do anything.
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Get a relative path.
		relPath, err := filepath.Rel(absRootDir, path)
		if err != nil {
			return err
		}

		// Create output file in e.g. out/news/2014/example.html
		htmlFile := replaceExt(filepath.Base(relPath), ".html")
		outPath := filepath.Join(*outDir, filepath.Dir(relPath), htmlFile)
		err = os.MkdirAll(filepath.Dir(outPath), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}

		// Read the markdown file.
		markdown, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		// Find an appropriate title.
		title := findNewsTitle(markdown)
		if title == "" {
			title = "Azul3D"
		} else {
			title += " - Azul3D"
		}

		log.Println(" -", relPath)
		return tmplRoot.ExecuteTemplate(out, newsTemplate, map[string]interface{} {
			"Title": title,
			"HTML": template.HTML(MarkdownNews(markdown)),
		})
	})
	return err
}
