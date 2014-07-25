package main

import (
	"bytes"
	"go/doc"
	"go/printer"
	"go/token"
	"html/template"
	"sort"
)

// Code for package documentation in the "Index" section.

type parsedIndexFunc struct {
	Name, Text string
	Decl, Doc  template.HTML
}

func parseIndexFunc(f *doc.Func, fset *token.FileSet) *parsedIndexFunc {
	textBuf := new(bytes.Buffer)
	printer.Fprint(textBuf, fset, f.Decl)

	return &parsedIndexFunc{
		Name: f.Name,
		Text: textBuf.String(),
		Decl: htmlDoc(textBuf.String()),
		Doc:  htmlDoc(f.Doc),
	}
}

type sortedIndexFuncs []*parsedIndexFunc

func (s sortedIndexFuncs) Len() int           { return len(s) }
func (s sortedIndexFuncs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedIndexFuncs) Less(i, j int) bool { return s[i].Name < s[j].Name }

func collectIndexFuncs(pkg *doc.Package, fset *token.FileSet) []*parsedIndexFunc {
	// Ignore functions that are returning a type defined in the package.
	typedFuncs := make(map[string]struct{}, len(pkg.Types)*6)
	for _, t := range pkg.Types {
		for _, f := range t.Funcs {
			typedFuncs[f.Name] = struct{}{}
		}
	}

	var funcs sortedIndexFuncs
	for _, f := range pkg.Funcs {
		_, typed := typedFuncs[f.Name]
		if !typed {
			funcs = append(funcs, parseIndexFunc(f, fset))
		}
	}
	sort.Sort(funcs)
	return funcs
}

type parsedIndexType struct {
	Name, Text     string
	Decl, Doc      template.HTML
	Funcs, Methods []*parsedIndexFunc
}

func parseIndexType(t *doc.Type, fset *token.FileSet) *parsedIndexType {
	textBuf := new(bytes.Buffer)
	printer.Fprint(textBuf, fset, t.Decl)

	parsed := &parsedIndexType{
		Name: t.Name,
		Text: textBuf.String(),
		Decl: htmlDoc(textBuf.String()),
		Doc:  htmlDoc(t.Doc),
	}
	for _, f := range t.Funcs {
		parsed.Funcs = append(parsed.Funcs, parseIndexFunc(f, fset))
	}
	for _, m := range t.Methods {
		parsed.Methods = append(parsed.Methods, parseIndexFunc(m, fset))
	}
	return parsed
}

type sortedIndexTypes []*parsedIndexType

func (s sortedIndexTypes) Len() int           { return len(s) }
func (s sortedIndexTypes) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedIndexTypes) Less(i, j int) bool { return s[i].Name < s[j].Name }

func collectIndexTypes(pkg *doc.Package, fset *token.FileSet) []*parsedIndexType {
	var types sortedIndexTypes
	for _, t := range pkg.Types {
		types = append(types, parseIndexType(t, fset))
	}
	sort.Sort(types)
	return types
}
