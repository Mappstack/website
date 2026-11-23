package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var symbolPattern = regexp.MustCompile(`<symbol id="([^"]+)"`)

var bemPattern = regexp.MustCompile(`\.([a-z][a-z0-9]*(?:(?:--|__|-)[a-z0-9]+)*(?:__|--)[a-z0-9-]+)`)
var attributeBracketPattern = regexp.MustCompile(`\[[^\]]*\]`)

type Design struct {
	Classes   map[string]bool
	Symbols   map[string]bool
	Contexts  map[string]map[string]bool
	FreeParts map[string]bool
}

func ReadDesign(directory string) (Design, error) {
	components, err := os.ReadFile(filepath.Join(directory, "components.css"))
	if err != nil {
		return Design{}, err
	}
	icons, err := os.ReadFile(filepath.Join(directory, "icons.svg"))
	if err != nil {
		return Design{}, err
	}
	design := Design{CollectSelectorClasses(string(components)), map[string]bool{}, map[string]map[string]bool{}, map[string]bool{}}
	for _, match := range symbolPattern.FindAllStringSubmatch(string(icons), -1) {
		design.Symbols[match[1]] = true
	}
	for _, rule := range ParseStyleRules(string(components), 1) {
		for _, alternative := range strings.Split(rule.Selector, ",") {
			matches := classSelectorPattern.FindAllStringSubmatch(alternative, -1)
			free := strings.ContainsAny(attributeBracketPattern.ReplaceAllString(alternative, ""), "+~")
			for index, match := range matches {
				if !strings.Contains(match[1], "__") {
					continue
				}
				design.FreeParts[match[1]] = design.FreeParts[match[1]] || free
				for _, before := range matches[:index] {
					if design.Contexts[match[1]] == nil {
						design.Contexts[match[1]] = map[string]bool{}
					}
					design.Contexts[match[1]][before[1]] = true
				}
			}
		}
	}
	for _, name := range []string{"readme.md", filepath.Join("skill", "readme.md"), "behaviors.js", "document.js"} {
		if text, err := os.ReadFile(filepath.Join(directory, name)); err == nil {
			for _, match := range bemPattern.FindAllStringSubmatch(string(text), -1) {
				design.Classes[match[1]] = true
			}
		}
	}
	return design, nil
}

func CheckClasses(root, designDirectory string, exclusions []string) ([]string, error) {
	design, err := ReadDesign(designDirectory)
	if err != nil {
		return nil, err
	}
	var problems []string
	err = WalkFiles(root, exclusions, markupExtensions, func(relative, extension, text string) {
		problems = append(problems, ReportFindings(relative, FindClassProblems(ParseMarkup(text, extension), design))...)
	})
	return problems, err
}

func FindClassProblems(markup *Markup, design Design) []Finding {
	designClasses := design.Classes
	var findings []Finding
	for _, element := range markup.Elements {
		for _, class := range element.Classes {
			base, _, _ := strings.Cut(class, "--")
			block, _, part := strings.Cut(base, "__")
			switch {
			case designClasses[class]:
			case !designClasses[block]:
			case part && !designClasses[base]:
				findings = append(findings, Finding{element.Line, fmt.Sprintf("class %s: %s is not a part of .%s: closed sets (Components: BEM)", class, class, block)})
			case class != base:
				findings = append(findings, Finding{element.Line, fmt.Sprintf("class %s: .%s has no such modifier: closed sets (Components: BEM)", class, base)})
			}
			if part && designClasses[block] && designClasses[base] && !design.FreeParts[base] && element.Parent != nil && element.IsWhole() && !HasPlace(element, block, design.Contexts[base]) {
				findings = append(findings, Finding{element.Line, fmt.Sprintf("class %s: no .%s around it: a part sits inside its block (Components: BEM)", class, block)})
			}
		}
	}
	return findings
}

func HasPlace(element *Element, block string, contexts map[string]bool) bool {
	for current := element; current != nil; current = current.Parent {
		if current.Has(block) {
			return true
		}
		for context := range contexts {
			if current.Has(context) {
				return true
			}
		}
	}
	return false
}
