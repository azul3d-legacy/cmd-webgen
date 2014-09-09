// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	bf "github.com/russross/blackfriday"
)

func mdRender(input []byte, sanitized bool) []byte {
	// set up the HTML renderer
	htmlFlags := 0
	htmlFlags |= bf.HTML_USE_XHTML
	htmlFlags |= bf.HTML_USE_SMARTYPANTS
	htmlFlags |= bf.HTML_SMARTYPANTS_FRACTIONS
	htmlFlags |= bf.HTML_SMARTYPANTS_LATEX_DASHES
	if sanitized {
		htmlFlags |= bf.HTML_SANITIZE_OUTPUT
	}
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

// mdFindTitle finds the title in the markdown input. It is expected to exist on
// the first line:
//  # Some Title Here
// The function then returns a string:
//  "Some Title Here"
func mdFindTitle(buf []byte) string {
	lineEnd := bytes.IndexAny(buf, "\n")
	if lineEnd == -1 {
		return ""
	}
	l := string(buf[:lineEnd])
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "#")
	return strings.TrimSpace(l)
}

// mdGenerate generates all of the Markdown files in the given folder path,
// rendering each one using the named template.
func mdGenerate(folder, tmpl string, sanitized bool) error {
	absPagesDir := filepath.Join(absRootDir, pagesDirName)
	dir := filepath.Join(absPagesDir, folder)
	// Generate each markdown page as needed.
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// If not a Markdown file, don't do anything.
		if filepath.Ext(path) != ".md" {
			return nil
		}

		// Open the file.
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// If not a regular file, don't do anything.
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Determine a path relative to the root directory.
		relPath, err := filepath.Rel(absPagesDir, path)
		if err != nil {
			return err
		}

		// Create output file in e.g. $OUT/news/2014/example.html
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

		// Read the Markdown file.
		markdown, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		// Find an appropriate title.
		title := mdFindTitle(markdown)
		if title == "" {
			title = mdDefaultTitle
		} else {
			title += mdAppendTitle
		}

		// Execute the template.
		log.Println(" -", relPath)
		return tmplRoot.ExecuteTemplate(out, tmpl, map[string]interface{}{
			"Title": title,
			"HTML":  template.HTML(mdRender(markdown, sanitized)),
		})
	})
}
