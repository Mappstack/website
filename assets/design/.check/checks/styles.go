package main

import (
	"fmt"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var variableValuePattern = regexp.MustCompile(`^var\((--[\w-]+)\s*(?:,[^)]*)?\)$`)
var rootStylePattern = regexp.MustCompile(`documentElement\.style(?:\.fontSize|\.setProperty\(\s*["']font-size)|\bhtml\.style\.fontSize`)
var queryFeaturePattern = regexp.MustCompile(`\(\s*([a-z0-9.-]+)`)
var userSettingPattern = regexp.MustCompile(`^(?:prefers-|forced-colors|not$)`)
var styleTagPattern = regexp.MustCompile(`(?is)<style([^>]*)>(.*?)</style>`)
var mediaAttributePattern = regexp.MustCompile(`(?i)(?:^|\s)media\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>"']+))`)
var viewportUnitPattern = regexp.MustCompile(`(?i)\d(?:[sld]?v(?:w|h|i|b|min|max)|cq(?:w|h|i|b|min|max))\b`)
var bareSizePattern = regexp.MustCompile(`^var\(--text-(\d+)\)$`)
var fontSizePattern = regexp.MustCompile(`(?:^|\s)var\(--text-(\d+)\)(?:/|\s|$)`)
var variableNamePattern = regexp.MustCompile(`var\((--[\w-]+)`)
var smoothScrollPattern = regexp.MustCompile(`behavior\s*:\s*["']smooth["']`)
var reducedMotionPattern = regexp.MustCompile(`prefers-reduced-motion`)
var tokenDefinitionPattern = regexp.MustCompile(`setProperty\(\s*["'](--[\w-]+)`)
var importPattern = regexp.MustCompile(`(?i)@import\s+(?:url\(\s*["']?([^"')\s]+)["']?\s*\)|"([^"]+)"|'([^']+)')([^;]*);`)
var importConditionPattern = regexp.MustCompile(`(?i)\blayer\b(?:\([^)]*\))?|\bsupports\((?:[^()]|\([^()]*\))*\)`)
var fontLengthPattern = regexp.MustCompile(`(?i)\d(?:px|r?em|ex|ch|lh|pt|pc|cm|mm|in|%)(?:\W|$)|var\(--text-`)
var parenthesesPattern = regexp.MustCompile(`\([^)]*\)`)
var combinatorPattern = regexp.MustCompile(`[\s>+~]+`)

var sizeAdjustProperties = map[string]bool{"text-size-adjust": true, "-webkit-text-size-adjust": true, "-moz-text-size-adjust": true, "-ms-text-size-adjust": true, "font-size-adjust": true, "size-adjust": true}
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
	Offset   int
}

type StyleRule struct {
	Selector     string
	Declarations []StyleDeclaration
	Line         int
	Contexts     []string
	Source       string
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
		for _, block := range styleTagPattern.FindAllStringSubmatchIndex(text, -1) {
			parsed := ParseStyleRules(text[block[4]:block[5]], LineOf(text, block[4]))
			if media := ReadMediaAttribute(text[block[2]:block[3]]); media != "" {
				parsed = WithMedia(parsed, media)
			}
			rules = append(rules, parsed...)
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
			if rule.Selector == "@font-face" && strings.ToLower(strings.TrimSpace(property)) != "size-adjust" {
				return
			}
			rule.Declarations = append(rule.Declarations, StyleDeclaration{strings.ToLower(strings.TrimSpace(property)), strings.TrimSpace(value), line(offset), offset})
		}
	}
	for index := 0; index < len(css); index++ {
		character := css[index]
		switch {
		case quote != 0:
			if character == '\\' {
				index++
			} else if character == quote {
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
			if strings.HasPrefix(prelude, "@") {
				prelude = strings.ToLower(prelude)
			}
			block := &styleBlock{prelude: prelude}
			switch {
			case strings.HasPrefix(prelude, "@font-face") && !(len(stack) > 0 && stack[len(stack)-1].skip):
				var contexts []string
				for _, open := range stack {
					contexts = append(contexts, open.prelude)
				}
				block.rule = &StyleRule{Selector: "@font-face", Line: line(start), Contexts: contexts}
			case strings.HasPrefix(prelude, "@keyframes") || len(stack) > 0 && stack[len(stack)-1].skip:
				block.skip = true
			case strings.HasPrefix(prelude, "@") && len(stack) > 0 && stack[len(stack)-1].rule != nil:
				var contexts []string
				for _, open := range stack {
					contexts = append(contexts, open.prelude)
				}
				block.rule = &StyleRule{Selector: stack[len(stack)-1].rule.Selector, Line: line(start), Contexts: append(contexts, prelude)}
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
	Inline   []StyleRule
	Text     string
}

func CheckStyles(root, designDirectory string, exclusions []string) ([]string, error) {
	design, err := ReadDesign(designDirectory)
	if err != nil {
		return nil, err
	}
	extensions := map[string]bool{".css": true, ".html": true, ".tmpl": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".go": true, ".py": true}
	var sources []StyleSource
	var links []StyleLink
	var pages []*Markup
	placed := map[string]string{}
	var problems []string
	err = WalkFiles(root, exclusions, extensions, func(relative, extension, text string) {
		source := StyleSource{Relative: relative, Text: text}
		source.Rules = ReadStyleRules(text, extension)
		for index := range source.Rules {
			source.Rules[index].Source = relative
		}
		links = append(links, ReadStyleImports(text, extension, relative)...)
		if markupExtensions[extension] {
			markup := ParseMarkup(text, extension)
			pages = append(pages, markup)
			for _, element := range markup.Elements {
				if link, found := ReadStyleLink(element, relative); found {
					links = append(links, link)
				}
				if value, present := element.Attribute("style"); present {
					for _, rule := range ParseStyleRules("x{"+value+"}", element.Line) {
						rule.Selector = fmt.Sprintf("style %s:%d", relative, element.Line)
						source.Inline = append(source.Inline, rule)
					}
				}
				for _, product := range FindProductClasses(element, design.Classes) {
					if placed[product] == "" && HasDesignClass(element, design.Classes) {
						placed[product] = fmt.Sprintf("%s:%d", relative, element.Line)
					}
				}
			}
			problems = append(problems, ReportFindings(relative, FindInlineStyleProblems(markup, design.Classes))...)
		}
		sources = append(sources, source)
	})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, source := range sources {
		known[source.Relative] = true
	}
	var everyRule []StyleRule
	for index := range sources {
		if strings.HasSuffix(sources[index].Relative, ".css") {
			for _, media := range ReadWrappingMedia(sources[index].Relative, links, known, map[string]bool{}) {
				sources[index].Rules = WithMedia(sources[index].Rules, media)
			}
		}
		everyRule = append(everyRule, sources[index].Rules...)
		everyRule = append(everyRule, sources[index].Inline...)
	}
	model := BuildTypeModel(everyRule, pages...)

	for _, source := range sources {
		var findings []Finding
		for _, rule := range source.Rules {
			findings = append(findings, FindRuleProblems(rule, placed, design.Classes)...)
		}
		if !strings.HasSuffix(source.Relative, ".css") {
			findings = append(findings, FindScriptProblems(source.Text)...)
		}
		findings = append(findings, FindTypeStepProblems(append(append([]StyleRule{}, source.Rules...), source.Inline...), model)...)
		problems = append(problems, ReportFindings(source.Relative, findings)...)
	}
	sort.Strings(problems)
	return problems, nil
}

type StyleLink struct {
	Href  string
	Media string
	From  string
	Page  bool
}

type TypeModel struct {
	Base    map[string]map[int]bool
	Size    map[string]bool
	Fluid   map[string]bool
	Verdict map[string]string
}

func ReadMediaAttribute(attributes string) string {
	if found := mediaAttributePattern.FindStringSubmatch(attributes); found != nil {
		return strings.TrimSpace(found[1] + found[2] + found[3])
	}
	return ""
}

func ReadStyleLink(element *Element, page string) (StyleLink, bool) {
	relation, _ := element.Attribute("rel")
	href, _ := element.Attribute("href")
	media, _ := element.Attribute("media")
	if !strings.EqualFold(element.Name, "link") || !strings.Contains(strings.ToLower(relation), "stylesheet") || strings.Contains(href, markupDynamic) || strings.Contains(href, "{{") || strings.Contains(href, "//") {
		return StyleLink{}, false
	}
	return StyleLink{href, strings.TrimSpace(media), page, true}, true
}

func ReadStyleImports(text, extension, relative string) []StyleLink {
	var links []StyleLink
	scan := func(css string) {
		for _, found := range importPattern.FindAllStringSubmatch(BlankMatches(css, commentPattern), -1) {
			href := found[1] + found[2] + found[3]
			if !strings.Contains(href, "//") && !strings.Contains(href, "{{") {
				links = append(links, StyleLink{href, strings.TrimSpace(importConditionPattern.ReplaceAllString(found[4], "")), relative, false})
			}
		}
	}
	if extension == ".css" {
		scan(text)
	} else if templateExtensions[extension] {
		for _, block := range styleTagPattern.FindAllStringSubmatch(text, -1) {
			scan(block[2])
		}
	}
	return links
}

func LinkReaches(link StyleLink, relative string, known map[string]bool) bool {
	path, _, _ := strings.Cut(link.Href, "#")
	path, _, _ = strings.Cut(path, "?")
	if path == "" {
		return false
	}
	if !strings.HasPrefix(path, "/") {
		if resolved := pathpkg.Join(pathpkg.Dir(link.From), path); known[resolved] {
			return relative == resolved
		}
	}
	path = pathpkg.Clean(path)
	for _, prefix := range []string{"/", "./", "../"} {
		for strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
		}
	}
	return path != "" && (relative == path || strings.HasSuffix(relative, "/"+path))
}

func ReadWrappingMedia(relative string, links []StyleLink, known, visited map[string]bool) []string {
	if visited[relative] {
		return nil
	}
	visited[relative] = true
	defer delete(visited, relative)
	var medias []string
	for _, link := range links {
		if !LinkReaches(link, relative, known) {
			continue
		}
		var inherited []string
		if !link.Page {
			inherited = ReadWrappingMedia(link.From, links, known, visited)
		}
		if link.Media == "" && len(inherited) == 0 {
			return nil
		}
		if link.Media != "" {
			medias = append(medias, link.Media)
		}
		medias = append(medias, inherited...)
	}
	return medias
}

func WithMedia(rules []StyleRule, media string) []StyleRule {
	wrapped := make([]StyleRule, len(rules))
	for index, rule := range rules {
		rule.Contexts = append([]string{"@media " + strings.ToLower(media)}, rule.Contexts...)
		wrapped[index] = rule
	}
	return wrapped
}

func SplitTopLevel(text string) []string {
	var parts []string
	depth, start := 0, 0
	for index, character := range text {
		switch character {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.Join(strings.Fields(text[start:index]), " "))
				start = index + 1
			}
		}
	}
	return append(parts, strings.Join(strings.Fields(text[start:]), " "))
}

func IsPrintAlternative(alternative string) bool {
	return alternative == "print" || strings.HasPrefix(alternative, "print ") || strings.HasPrefix(alternative, "only print")
}

func IsSizeMediaQuery(prelude string) bool {
	for _, alternative := range SplitTopLevel(strings.ToLower(prelude)) {
		if IsPrintAlternative(alternative) {
			continue
		}
		for _, feature := range queryFeaturePattern.FindAllStringSubmatch(alternative, -1) {
			if !userSettingPattern.MatchString(feature[1]) {
				return true
			}
		}
	}
	return false
}

func IsPrintRule(rule StyleRule) bool {
	for _, context := range rule.Contexts {
		prelude, found := strings.CutPrefix(context, "@media")
		if !found {
			continue
		}
		every := true
		for _, alternative := range SplitTopLevel(prelude) {
			every = every && IsPrintAlternative(alternative)
		}
		if every {
			return true
		}
	}
	return false
}

func IsSizeQuery(rule StyleRule) bool {
	if IsPrintRule(rule) {
		return false
	}
	for _, context := range rule.Contexts {
		if strings.HasPrefix(context, "@container") {
			return true
		}
		if prelude, found := strings.CutPrefix(context, "@media"); found && IsSizeMediaQuery(prelude) {
			return true
		}
	}
	return false
}

func ReadBareSize(property, value string) (int, bool) {
	pattern := bareSizePattern
	if property == "font" {
		pattern = fontSizePattern
	}
	if found := pattern.FindStringSubmatch(value); found != nil {
		pixels, _ := strconv.Atoi(found[1])
		return pixels, true
	}
	return 0, false
}

func NameSizeProperty(property string) string {
	if property == "font" {
		return "font-size"
	}
	return property
}

func NormalizeSelector(alternative string) string {
	var normalized []rune
	depth := 0
	for _, character := range strings.Join(strings.Fields(alternative), " ") {
		switch character {
		case '[':
			depth++
		case ']':
			depth--
		}
		combinator := depth == 0 && strings.ContainsRune(">+~", character)
		last := len(normalized) - 1
		if combinator && last >= 0 && normalized[last] == ' ' {
			normalized = normalized[:last]
		}
		if character == ' ' && last >= 0 && depth == 0 && strings.ContainsRune(">+~", normalized[last]) {
			continue
		}
		normalized = append(normalized, character)
	}
	return string(normalized)
}

func BuildTypeModel(rules []StyleRule, pages ...*Markup) TypeModel {
	model := TypeModel{map[string]map[int]bool{}, map[string]bool{"font-size": true}, map[string]bool{}, map[string]string{}}
	weak := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, rule := range rules {
			for _, declaration := range rule.Declarations {
				property := declaration.Property
				if strings.HasPrefix(property, "--") && !model.Fluid[property] && viewportUnitPattern.MatchString(declaration.Value) {
					model.Fluid[property], changed = true, true
				}
				if weak[property] && !model.Size[property] && (fontLengthPattern.MatchString(declaration.Value) || viewportUnitPattern.MatchString(declaration.Value) || ReadsSize(declaration.Value, model)) {
					model.Size[property], changed = true, true
				}
				for _, name := range variableNamePattern.FindAllStringSubmatch(declaration.Value, -1) {
					if (property == "font" || weak[property]) && !model.Size[name[1]] && !weak[name[1]] && !HasTokenPrefix(name[1]) {
						weak[name[1]], changed = true, true
					}
					if model.Size[property] && !weak[property] && (!model.Size[name[1]] || weak[name[1]]) && !HasTokenPrefix(name[1]) {
						model.Size[name[1]], weak[name[1]], changed = true, false, true
					}
					if strings.HasPrefix(property, "--") && model.Fluid[name[1]] && !model.Fluid[property] {
						model.Fluid[property], changed = true, true
					}
				}
			}
		}
	}
	model.Size["font"] = true
	type override struct{ source, contexts, key string }
	last := map[override]int{}
	for _, rule := range rules {
		if IsSizeQuery(rule) || IsPrintRule(rule) {
			continue
		}
		for _, declaration := range rule.Declarations {
			if !model.Size[declaration.Property] {
				continue
			}
			pixels, found := ReadBareSize(declaration.Property, declaration.Value)
			if !found {
				pixels = -1
			}
			for _, alternative := range SplitTopLevel(rule.Selector) {
				key := NormalizeSelector(alternative) + "|" + NameSizeProperty(declaration.Property)
				last[override{rule.Source, strings.Join(rule.Contexts, "\n"), key}] = pixels
			}
		}
	}
	for overriding, pixels := range last {
		if model.Base[overriding.key] == nil {
			model.Base[overriding.key] = map[int]bool{}
		}
		model.Base[overriding.key][pixels] = true
	}
	JudgeSteps(rules, pages, &model)
	return model
}

func ReadsSize(value string, model TypeModel) bool {
	for _, name := range variableNamePattern.FindAllStringSubmatch(value, -1) {
		if model.Size[name[1]] {
			return true
		}
	}
	return false
}

func FindTypeStepProblems(rules []StyleRule, model TypeModel) []Finding {
	var findings []Finding
	for _, rule := range rules {
		for _, declaration := range rule.Declarations {
			property, value := declaration.Property, declaration.Value
			report := func(format string, arguments ...any) {
				findings = append(findings, Finding{declaration.Line, fmt.Sprintf(format, arguments...) + ": Principles 8: one size on every device, only display type (--text-40 and up) steps, once, at 900, smaller below it"})
			}
			isSize := model.Size[property]
			if isSize && viewportUnitPattern.MatchString(value) {
				report("%s %s follows the viewport", property, value)
			}
			for _, name := range variableNamePattern.FindAllStringSubmatch(value, -1) {
				if isSize && model.Fluid[name[1]] {
					report("%s uses %s, which follows the viewport", property, name[1])
				}
			}
			if !IsSizeQuery(rule) {
				continue
			}
			_, bare := ReadBareSize(property, value)
			tokenPixels, token := ReadBareSize("font-size", value)
			switch {
			case property == "all":
				report("all in a query resets the size")
			case property == "zoom":
				report("zoom %s in a query rescales the type", value)
			case sizeAdjustProperties[property]:
				report("%s in a query rescales the type", property)
			case isSize && !bare:
				report("%s %s in a query: only a bare --text token may set a size there", property, value)
			case strings.HasPrefix(property, "--") && !isSize && token && tokenPixels < 40:
				report("%s carries a size below --text-40 in a query", property)
			case isSize:
				if reason := model.Verdict[DeclarationKey(rule, declaration)]; reason != "" {
					report("%s %s", property, reason)
				}
			}
		}
	}
	return findings
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
