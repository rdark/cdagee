package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"
)

type outputFormat int

const (
	formatText outputFormat = iota
	formatJSON
	formatGoTemplate
	formatGoTemplateFile
)

type commonFlags struct {
	root       string
	format     outputFormat
	tmpl       *template.Template
	jsonFlag   bool
	outputFlag string
}

func addCommonFlags(fs *flag.FlagSet) *commonFlags {
	cf := &commonFlags{}
	fs.StringVar(&cf.root, "root", ".", "root directory to scan for targets")
	fs.BoolVar(&cf.jsonFlag, "json", false, "output in JSON format (alias for -o json)")
	fs.StringVar(&cf.outputFlag, "output", "", "output format: text, json, go-template=<tmpl>, go-template-file=<path>")
	fs.StringVar(&cf.outputFlag, "o", "", "")
	return cf
}

// resolveOutput validates the --json / --output flags and parses any template.
// Must be called after fs.Parse().
func resolveOutput(cf *commonFlags, fs *flag.FlagSet) {
	// Detect conflict: both --json and --output explicitly set.
	jsonExplicit := false
	outputExplicit := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "json":
			jsonExplicit = true
		case "output", "o":
			outputExplicit = true
		}
	})
	if jsonExplicit && outputExplicit {
		fatalf("--json and --output cannot be used together")
	}

	// --json is shorthand for -o json.
	if cf.jsonFlag {
		cf.format = formatJSON
		return
	}

	if cf.outputFlag == "" || cf.outputFlag == "text" {
		cf.format = formatText
		return
	}

	if cf.outputFlag == "json" {
		cf.format = formatJSON
		return
	}

	if tmplStr, ok := strings.CutPrefix(cf.outputFlag, "go-template="); ok {
		t, err := template.New("out").Funcs(templateFuncs()).Parse(tmplStr)
		if err != nil {
			fatalf("invalid template: %v", err)
		}
		cf.format = formatGoTemplate
		cf.tmpl = t
		return
	}

	if path, ok := strings.CutPrefix(cf.outputFlag, "go-template-file="); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			fatalf("reading template file: %v", err)
		}
		t, err := template.New("out").Funcs(templateFuncs()).Parse(string(data))
		if err != nil {
			fatalf("invalid template: %v", err)
		}
		cf.format = formatGoTemplateFile
		cf.tmpl = t
		return
	}

	fatalf("unknown output format: %q", cf.outputFlag)
}

// writeOutput renders v as JSON or via a Go template. Returns true if output
// was handled (caller should skip text rendering). Returns false for formatText.
func (cf *commonFlags) writeOutput(v any) bool {
	switch cf.format {
	case formatText:
		return false
	case formatJSON:
		if err := writeJSON(v); err != nil {
			fatalf("%v", err)
		}
		return true
	case formatGoTemplate, formatGoTemplateFile:
		if err := cf.tmpl.Execute(os.Stdout, v); err != nil {
			fatalf("template: %v", err)
		}
		_, _ = fmt.Fprintln(os.Stdout)
		return true
	default:
		return false
	}
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"toJSON": func(v any) (string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"toPrettyJSON": func(v any) (string, error) {
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"join": strings.Join,
	}
}

// addTagsFlag registers a --tags flag on the given FlagSet and returns a
// function that parses the comma-separated value into a string slice.
func addTagsFlag(fs *flag.FlagSet) func() []string {
	var tagsFlag string
	fs.StringVar(&tagsFlag, "tags", "", "only include targets matching any listed tag (comma-separated)")
	return func() []string {
		if tagsFlag == "" {
			return nil
		}
		return strings.Split(tagsFlag, ",")
	}
}

func checkNoArgs(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fatalf("%s: unexpected arguments: %v", fs.Name(), fs.Args())
	}
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func splitErrors(err error) []string {
	if err == nil {
		return nil
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		var msgs []string
		for _, e := range multi.Unwrap() {
			msgs = append(msgs, e.Error())
		}
		return msgs
	}
	return []string{err.Error()}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
