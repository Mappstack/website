package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var variableValuePattern = regexp.MustCompile(`^var\((--[\w-]+)\s*(?:,[^)]*)?\)$`)
var rootStylePattern = regexp.MustCompile(`documentElement\.style(?:\.fontSize|\.setProperty\(\s*["']font-size)|\bhtml\.style\.fontSize`)
var smoothScrollPattern = regexp.MustCompile(`behavior\s*:\s*["']smooth["']`)
var reducedMotionPattern = regexp.MustCompile(`prefers-reduced-motion`)
var tokenDefinitionPattern = regexp.MustCompile(`setProperty\(\s*["'](--[\w-]+)`)
var parenthesesPattern = regexp.MustCompile(`\([^)]*\)`)
var combinatorPattern = regexp.MustCompile(`[\s>+~]+`)

var tokenPrefixes = []string{"--color-", "--space-", "--text-", "--radius-", "--size-", "--width-", "--shadow-", "--z-", "--duration-", "--delay-", "--ease-", "--opacity-", "--font-", "--weight-", "--leading-", "--tracking-", "--page-", "--card-", "--frost", "--press-", "--reading-", "--scale-", "--ratio-", "--drift-"}
var colorProperties = map[string]bool{"color": true, "background-color": true, "border-color": true, "outline-color": true, "fill": true, "stroke": true, "caret-color": true, "accent-color": true, "text-decoration-color": true, "border-top-color": true, "border-right-color": true, "border-bottom-color": true, "border-left-color": true}
var lengthProperties = map[string]bool{"font-size": true, "margin": true, "margin-top": true, "margin-right": true, "margin-bottom": true, "margin-left": true, "padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true, "gap": true, "row-gap": true, "column-gap": true, "width": true, "height": true, "min-width": true, "min-height": true, "max-width": true, "max-height": true, "border-radius": true, "letter-spacing": true, "z-index": true, "opacity": true}
var profileColors = map[string]bool{"--page-background": true, "--card-border-color": true}
var layoutProperties = map[string]bool{"width": true, "min-width": true, "max-width": true, "height": true, "min-height": true, "max-height": true, "display": true, "position": true, "order": true, "top": true, "right": true, "bottom": true, "left": true, "gap": true, "row-gap": true, "column-gap": true, "text-align": true, "aspect-ratio": true}
var layoutPrefixes = []string{"margin", "flex", "grid", "align-", "justify-", "place-", "inset"}
var outlineNonePattern = regexp.MustCompile(`^(none|0|0px|0 none)\b`)
var baseReferencePattern = regexp.MustCompile(`--base-[\w-]+`)

type StyleDeclaration struct {
	Property string
	Value    string
	Line     int
}

type StyleRule struct {
	Selector     string
	Declarations []StyleDeclaration
	Line         int
	Contexts     []string
}

type styleBlock struct {
	prelude string
	rule    *StyleRule
	skip    bool
}

func ReadStyleRules(text, extension string) []StyleRule {
	if extension == ".css" {
		return ParseStyleRules(text, 1)
	}
	var rules []StyleRule
	if templateExtensions[extension] {
		for _, block := range styleBlockPattern.FindAllStringSubmatchIndex(text, -1) {
			rules = append(rules, ParseStyleRules(text[block[2]:block[3]], LineOf(text, block[2]))...)
		}
	}
	return rules
}

func ParseStyleRules(css string, firstLine int) []StyleRule {
	css = BlankMatches(css, commentPattern)
	var rules []StyleRule
	var stack []*styleBlock
	start, parentheses := 0, 0
	var quote byte
	line := func(offset int) int { return firstLine + strings.Count(css[:offset], "\n") }
	finish := func(end int) {
		if len(stack) == 0 || stack[len(stack)-1].rule == nil {
			return
		}
		declaration := css[start:end]
		property, value, found := strings.Cut(declaration, ":")
		if found && strings.TrimSpace(property) != "" {
			offset := start + len(declaration) - len(strings.TrimLeft(declaration, " \t\r\n"))
			rule := stack[len(stack)-1].rule
			rule.Declarations = append(rule.Declarations, StyleDeclaration{strings.ToLower(strings.TrimSpace(property)), strings.TrimSpace(value), line(offset)})
		}
	}
	for index := 0; index < len(css); index++ {
		character := css[index]
		switch {
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case character == '"' || character == '\'':
			quote = character
		case character == '(':
			parentheses++
		case character == ')':
			parentheses--
		case parentheses > 0:
		case character == '{':
			prelude := strings.TrimSpace(css[start:index])
			block := &styleBlock{prelude: prelude}
			switch {
			case strings.HasPrefix(prelude, "@keyframes") || strings.HasPrefix(prelude, "@font-face") || len(stack) > 0 && stack[len(stack)-1].skip:
				block.skip = true
			case strings.HasPrefix(prelude, "@"):
			default:
				var contexts []string
				for _, open := range stack {
					contexts = append(contexts, open.prelude)
				}
				block.rule = &StyleRule{Selector: prelude, Line: line(start + len(css[start:index]) - len(strings.TrimLeft(css[start:index], " \t\r\n"))), Contexts: contexts}
			}
			stack = append(stack, block)
			start = index + 1
		case character == ';':
			finish(index)
			start = index + 1
		case character == '}':
			finish(index)
			if len(stack) > 0 {
				if block := stack[len(stack)-1]; block.rule != nil {
					rules = append(rules, *block.rule)
				}
				stack = stack[:len(stack)-1]
			}
			start = index + 1
		}
	}
	return rules
}

func IsLayoutProperty(property string) bool {
	if layoutProperties[property] || strings.HasPrefix(property, "--") {
		return true
	}
	for _, prefix := range layoutPrefixes {
		if strings.HasPrefix(property, prefix) {
			return true
		}
	}
	return false
}

func FindLastCompound(alternative string) string {
	alternative = parenthesesPattern.ReplaceAllString(attributeBracketPattern.ReplaceAllString(alternative, "[]"), "()")
	parts := combinatorPattern.Split(strings.TrimSpace(alternative), -1)
	return parts[len(parts)-1]
}

type StyleSource struct {
	Relative string
	Rules    []StyleRule
	Text     string
}

func CheckStyles(root, designDirectory string, exclusions []string) ([]string, error) {
	design, err := ReadDesign(designDirectory)
	if err != nil {
		return nil, err
	}
	extensions := map[string]bool{".css": true, ".html": true, ".tmpl": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".go": true, ".py": true}
	var sources []StyleSource
	placed := map[string]string{}
	var problems []string
	err = WalkFiles(root, exclusions, extensions, func(relative, extension, text string) {
		source := StyleSource{Relative: relative, Text: text}
		source.Rules = ReadStyleRules(text, extension)
		sources = append(sources, source)
		if markupExtensions[extension] {
			markup := ParseMarkup(text, extension)
			for _, element := range markup.Elements {
				for _, product := range FindProductClasses(element, design.Classes) {
					if placed[product] == "" && HasDesignClass(element, design.Classes) {
						placed[product] = fmt.Sprintf("%s:%d", relative, element.Line)
					}
				}
			}
			problems = append(problems, ReportFindings(relative, FindInlineStyleProblems(markup, design.Classes))...)
		}
	})
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		var findings []Finding
		for _, rule := range source.Rules {
			findings = append(findings, FindRuleProblems(rule, placed, design.Classes)...)
		}
		if !strings.HasSuffix(source.Relative, ".css") {
			findings = append(findings, FindScriptProblems(source.Text)...)
		}
		problems = append(problems, ReportFindings(source.Relative, findings)...)
	}
	sort.Strings(problems)
	return problems, nil
}

func FindProductClasses(element *Element, designClasses map[string]bool) []string {
	var classes []string
	for _, class := range element.Classes {
		if !designClasses[class] && !strings.HasPrefix(class, "is-") {
			classes = append(classes, class)
		}
	}
	return classes
}

func HasDesignClass(element *Element, designClasses map[string]bool) bool {
	for _, class := range element.Classes {
		if designClasses[class] && !strings.HasPrefix(class, "is-") {
			return true
		}
	}
	return false
}

func FindInlineStyleProblems(markup *Markup, designClasses map[string]bool) []Finding {
	var findings []Finding
	for _, element := range markup.Elements {
		value, present := element.Attribute("style")
		if !present || !HasDesignClass(element, designClasses) || strings.Contains(value, markupDynamic) || strings.Contains(value, "${") {
			continue
		}
		for _, rule := range ParseStyleRules("x{"+value+"}", element.Line) {
			for _, declaration := range rule.Declarations {
				if !IsLayoutProperty(declaration.Property) {
					findings = append(findings, Finding{element.Line, fmt.Sprintf("inline %s on a design element: Principles 11: a product sets layout only (margin, flex, grid, width) on a design element", declaration.Property)})
				}
			}
		}
	}
	return findings
}

func FindRuleProblems(rule StyleRule, placed map[string]string, designClasses map[string]bool) []Finding {
	var findings []Finding
	inForced, inMotion := false, false
	for _, context := range rule.Contexts {
		inForced = inForced || strings.Contains(context, "forced-colors")
		inMotion = inMotion || strings.Contains(context, "no-preference")
	}
	restyled := map[string][]string{}
	for _, declaration := range rule.Declarations {
		property, value := declaration.Property, declaration.Value
		for _, alternative := range strings.Split(rule.Selector, ",") {
			for _, match := range classSelectorPattern.FindAllStringSubmatch(FindLastCompound(alternative), -1) {
				if _, shared := placed[match[1]]; shared && !IsLayoutProperty(property) {
					restyled[match[1]] = append(restyled[match[1]], property)
				}
			}
		}
		report := func(format string, arguments ...any) {
			findings = append(findings, Finding{declaration.Line, fmt.Sprintf(format, arguments...)})
		}
		for _, alternative := range strings.Split(rule.Selector, ",") {
			alternative = strings.TrimSpace(alternative)
			if (alternative == "html" || alternative == ":root" || alternative == "html:root") && (property == "font-size" || property == "font") {
				report("%s sets the root %s: Principles 10: nothing sets the root font size", alternative, property)
			}
			if property == "outline" && outlineNonePattern.MatchString(value) || property == "outline-style" && value == "none" || property == "outline-width" && value == "0" {
				if !strings.Contains(alternative, `[tabindex="-1"]`) && !strings.Contains(alternative, "not(:focus-visible)") {
					report("%s removes the outline: Principles 3: one focus recipe, focus is an outline", alternative)
				}
			}
			if property == "box-shadow" && strings.Contains(alternative, ":focus") {
				report("%s draws focus with box-shadow: Principles 3: one focus recipe, focus is an outline", alternative)
			}
		}
		if property == "box-sizing" && value == "content-box" {
			report("box-sizing content-box: Components: every element is border-box")
		}
		if property == "forced-color-adjust" && value == "none" && !inForced {
			report("forced-color-adjust none: Principles 10: forced colors keep every control visible")
		}
		if property == "scroll-behavior" && value == "smooth" && !inMotion {
			report("scroll-behavior smooth outside prefers-reduced-motion: no-preference: Principles 10: reduced motion stops all movement")
		}
		if property == "color-scheme" && (!strings.Contains(value, "light") || !strings.Contains(value, "dark") || strings.Contains(value, "only")) {
			report("color-scheme %s: Patterns 9: light and dark follow the system", value)
		}
		if strings.HasPrefix(property, "--") {
			for _, prefix := range tokenPrefixes {
				if strings.HasPrefix(property, prefix) {
					if strings.HasPrefix(property, "--page-") || strings.HasPrefix(property, "--card-") {
						report("%s set by a product class: Principles 11: a profile choice is the design's, a product does not restyle it", property)
					} else {
						report("%s defined in product CSS: Principles 4 and 5: a value that is not a token does not exist, one name has one meaning", property)
					}
					break
				}
			}
		}
		if found := variableValuePattern.FindStringSubmatch(value); found != nil {
			switch {
			case strings.HasPrefix(found[1], "--base-"):
			case strings.HasPrefix(found[1], "--color-") && lengthProperties[property]:
				report("%s used as %s: Tokens: every color starts with --color- and a size never does", found[1], property)
			case colorProperties[property] && !profileColors[found[1]] && !strings.HasPrefix(found[1], "--color-") && !strings.HasPrefix(found[1], "--opacity-") && HasTokenPrefix(found[1]):
				report("%s used as %s: Tokens: every color starts with --color- and a size never does", found[1], property)
			}
		}
		for _, match := range baseReferencePattern.FindAllString(value, -1) {
			report("%s used in a product: Tokens: --base-* is never used by products", match)
		}
	}
	var classes []string
	for class := range restyled {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	for _, class := range classes {
		findings = append(findings, Finding{rule.Line, fmt.Sprintf(".%s sets %s on a design element (%s): Principles 11: a product class on a design element sets layout only (margin, flex, grid, width)", class, strings.Join(Deduplicate(restyled[class]), ", "), placed[class])})
	}
	return findings
}

func HasTokenPrefix(name string) bool {
	for _, prefix := range tokenPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func FindScriptProblems(text string) []Finding {
	var findings []Finding
	for _, match := range rootStylePattern.FindAllStringIndex(text, -1) {
		findings = append(findings, Finding{LineOf(text, match[0]), "script sets the root font size: Principles 10: nothing sets the root font size"})
	}
	if !reducedMotionPattern.MatchString(text) {
		for _, match := range smoothScrollPattern.FindAllStringIndex(text, -1) {
			findings = append(findings, Finding{LineOf(text, match[0]), "smooth scroll without prefers-reduced-motion: Principles 10: reduced motion stops all movement"})
		}
	}
	for _, match := range tokenDefinitionPattern.FindAllStringSubmatchIndex(text, -1) {
		if name := text[match[2]:match[3]]; HasTokenPrefix(name) {
			findings = append(findings, Finding{LineOf(text, match[0]), fmt.Sprintf("script sets %s: Principles 4 and 5: a value that is not a token does not exist, one name has one meaning", name)})
		}
	}
	return findings
}
