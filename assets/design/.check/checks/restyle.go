package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func CheckRestyle(productDirectory, designDirectory string, exclusions []string) ([]string, error) {
	components, err := os.ReadFile(filepath.Join(designDirectory, "components.css"))
	if err != nil {
		return nil, err
	}
	designClasses := CollectSelectorClasses(string(components))
	var problems []string
	err = WalkFiles(productDirectory, exclusions, map[string]bool{".css": true, ".html": true, ".tmpl": true}, func(relative, extension, text string) {
		for _, rule := range ReadStyleRules(text, extension) {
			var restyled []string
			for _, match := range classSelectorPattern.FindAllStringSubmatch(rule.Selector, -1) {
				if designClasses[match[1]] && !strings.HasPrefix(match[1], "is-") {
					restyled = append(restyled, "."+match[1])
				}
			}
			if len(restyled) > 0 {
				problems = append(problems, fmt.Sprintf("%s:%d: styles %s, which the design system owns: change it there", relative, rule.Line, strings.Join(Deduplicate(restyled), ", ")))
			}
		}
	})
	sort.Strings(problems)
	return problems, err
}
