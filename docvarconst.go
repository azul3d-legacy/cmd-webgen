// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"go/doc"
	"go/printer"
	"go/token"
	"sort"
)

// Code for package documentation in the "Constants" or "Variables" section.

type parsedValue struct {
	Name, Text, Doc string
}

type sortedValues []*parsedValue

func (s sortedValues) Len() int           { return len(s) }
func (s sortedValues) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedValues) Less(i, j int) bool { return s[i].Name < s[j].Name }

func parseValue(v *doc.Value, fset *token.FileSet) *parsedValue {
	textBuf := new(bytes.Buffer)
	printer.Fprint(textBuf, fset, v.Decl)

	return &parsedValue{
		Text: textBuf.String(),
		Doc:  v.Doc,
	}
}

func collectValues(pkg *doc.Package, fset *token.FileSet) (vars, consts []*parsedValue) {
	for _, v := range pkg.Consts {
		consts = append(consts, parseValue(v, fset))
	}
	for _, v := range pkg.Vars {
		vars = append(vars, parseValue(v, fset))
	}
	sort.Sort(sortedValues(consts))
	sort.Sort(sortedValues(vars))
	return
}
