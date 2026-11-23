package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Renames struct {
	Tokens  map[string]string `json:"tokens"`
	Classes map[string]string `json:"classes"`
}

func ApplyRenames(renamesPath, productDirectory string, skipped []string) error {
	content, err := os.ReadFile(renamesPath)
	if err != nil {
		return err
	}
	var renames Renames
	if err := json.Unmarshal(content, &renames); err != nil {
		return err
	}
	if len(renames.Tokens) == 0 && len(renames.Classes) == 0 {
		return nil
	}
	type Replacement struct {
		Pattern *regexp.Regexp
		Value   string
	}
	var replacements []Replacement
	for old, renamed := range renames.Tokens {
		replacements = append(replacements, Replacement{regexp.MustCompile(regexp.QuoteMeta(old) + `\b(?:[^-\w]|$)`), renamed})
	}
	for old, renamed := range renames.Classes {
		if renamed == "" {
			replacements = append(replacements, Replacement{regexp.MustCompile(`\s+` + regexp.QuoteMeta(old) + `([\s"'])`), "${1}"}, Replacement{regexp.MustCompile(`(["'])` + regexp.QuoteMeta(old) + `(?:\s+|(["']))`), "${1}${2}"})
			continue
		}
		replacements = append(replacements, Replacement{regexp.MustCompile(`([."'\s])` + regexp.QuoteMeta(old) + `([\s"'.:,>+~\[{)]|$)`), "${1}" + renamed + "${2}"})
	}
	return filepath.WalkDir(productDirectory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && path != productDirectory || entry.Name() == "node_modules" || entry.Name() == "vendor" || IsSkipped(productDirectory, path, skipped) {
				return filepath.SkipDir
			}
			return nil
		}
		extension := filepath.Ext(path)
		if extension != ".css" && extension != ".js" && extension != ".html" && extension != ".tmpl" && extension != ".go" {
			return nil
		}
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(original)
		for _, replacement := range replacements {
			if strings.HasPrefix(replacement.Value, "--") {
				text = replacement.Pattern.ReplaceAllStringFunc(text, func(found string) string {
					return replacement.Value + found[len(found)-TrailingLength(found):]
				})
				continue
			}
			text = replacement.Pattern.ReplaceAllString(text, replacement.Value)
		}
		if text == string(original) {
			return nil
		}
		return os.WriteFile(path, []byte(text), entry.Type().Perm()|0o644)
	})
}

func TrailingLength(found string) int {
	last := found[len(found)-1]
	if last == '-' || last == '_' || last >= '0' && last <= '9' || last >= 'a' && last <= 'z' || last >= 'A' && last <= 'Z' {
		return 0
	}
	return 1
}

func IsSkipped(productDirectory, path string, skipped []string) bool {
	relative, _ := filepath.Rel(productDirectory, path)
	for _, directory := range skipped {
		if relative == filepath.Clean(directory) {
			return true
		}
	}
	return false
}
