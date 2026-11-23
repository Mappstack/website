package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var variableReferencePattern = regexp.MustCompile(`var\((--[\w-]+)`)
var variableDefinitionPattern = regexp.MustCompile(`(--[\w-]+)\s*:`)
var propertySetterPattern = regexp.MustCompile(`setProperty\(\s*["'](--[\w-]+)`)
var classSelectorPattern = regexp.MustCompile(`\.([a-zA-Z][\w-]*)`)
var classAttributePattern = regexp.MustCompile(`class(?:Name)?\s*=\s*["']([^"']*)["']`)
var classListPattern = regexp.MustCompile(`classList\.(?:add|remove|toggle|contains|replace)\(([^)]*)\)`)
var quotedPattern = regexp.MustCompile(`["']([\w-]+)["']`)

func CheckUsage(productDirectory, designDirectory, previousComponents string) ([]string, error) {
	definitions := map[string]bool{}
	references := map[string][]string{}
	usedClasses := map[string][]string{}
	err := WalkFiles(productDirectory, nil, map[string]bool{".css": true, ".js": true, ".html": true, ".tmpl": true, ".go": true}, func(relative, extension, text string) {
		for _, match := range variableDefinitionPattern.FindAllStringSubmatch(text, -1) {
			definitions[match[1]] = true
		}
		for _, match := range propertySetterPattern.FindAllStringSubmatch(text, -1) {
			definitions[match[1]] = true
		}
		for _, match := range variableReferencePattern.FindAllStringSubmatchIndex(text, -1) {
			references[text[match[2]:match[3]]] = append(references[text[match[2]:match[3]]], fmt.Sprintf("%s:%d", relative, LineOf(text, match[0])))
		}
		for _, use := range CollectClassUses(text) {
			usedClasses[use.Name] = append(usedClasses[use.Name], fmt.Sprintf("%s:%d", relative, LineOf(text, use.Offset)))
		}
	})
	if err != nil {
		return nil, err
	}
	designDefinitions, err := CollectDesignDefinitions(designDirectory)
	if err != nil {
		return nil, err
	}
	for name := range designDefinitions {
		definitions[name] = true
	}
	var problems []string
	for name, files := range references {
		if !definitions[name] {
			problems = append(problems, fmt.Sprintf("usage: %s is not defined, used in %s", name, strings.Join(Deduplicate(files), ", ")))
		}
	}
	if previousComponents != "" {
		previous, err := os.ReadFile(previousComponents)
		if err != nil {
			return nil, err
		}
		current, err := os.ReadFile(filepath.Join(designDirectory, "components.css"))
		if err != nil {
			return nil, err
		}
		currentClasses := CollectSelectorClasses(string(current))
		for name := range CollectSelectorClasses(string(previous)) {
			if !currentClasses[name] && len(usedClasses[name]) > 0 {
				problems = append(problems, fmt.Sprintf("usage: class %s was dropped, used in %s", name, strings.Join(Deduplicate(usedClasses[name]), ", ")))
			}
		}
	}
	sort.Strings(problems)
	return problems, nil
}

func CollectDesignDefinitions(designDirectory string) (map[string]bool, error) {
	definitions := map[string]bool{}
	paths, err := filepath.Glob(filepath.Join(designDirectory, "tokens", "*.css"))
	if err != nil {
		return nil, err
	}
	profiles, _ := filepath.Glob(filepath.Join(designDirectory, "profiles", "*.css"))
	paths = append(paths, append(profiles, filepath.Join(designDirectory, "components.css"), filepath.Join(designDirectory, "profile.css"))...)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, match := range variableDefinitionPattern.FindAllStringSubmatch(string(content), -1) {
			definitions[match[1]] = true
		}
	}
	return definitions, nil
}

func CollectSelectorClasses(css string) map[string]bool {
	classes := map[string]bool{}
	for _, rule := range ParseStyleRules(css, 1) {
		for _, match := range classSelectorPattern.FindAllStringSubmatch(rule.Selector, -1) {
			classes[match[1]] = true
		}
	}
	return classes
}

type ClassUse struct {
	Name   string
	Offset int
}

func LineOf(text string, offset int) int {
	return strings.Count(text[:offset], "\n") + 1
}

func CollectClassUses(text string) []ClassUse {
	var uses []ClassUse
	for _, match := range classAttributePattern.FindAllStringSubmatchIndex(text, -1) {
		for _, name := range strings.Fields(text[match[2]:match[3]]) {
			uses = append(uses, ClassUse{name, match[0]})
		}
	}
	for _, match := range classListPattern.FindAllStringSubmatchIndex(text, -1) {
		for _, quoted := range quotedPattern.FindAllStringSubmatch(text[match[2]:match[3]], -1) {
			uses = append(uses, ClassUse{quoted[1], match[0]})
		}
	}
	return uses
}

func Deduplicate(values []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	return unique
}
