package main

import (
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var markupExtensions = map[string]bool{".html": true, ".tmpl": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".go": true, ".py": true}
var templateExtensions = map[string]bool{".html": true, ".tmpl": true}

var markupHiddenPattern = regexp.MustCompile(`(?is)<script\b.*?</script>|<style\b.*?</style>|<noscript\b.*?</noscript>|<!--.*?-->`)
var markupActionPattern = regexp.MustCompile(`^\{\{-?\s*(if|else|end|with|range|block|define|template)\b`)
var markupValuePattern = regexp.MustCompile(`(?s)\{\{.*?\}\}|\$\{[^}]*\}`)
var markupVerbPattern = regexp.MustCompile(`%(?:\[\d+\])?[sdvqxf]`)
var markupFormatPattern = regexp.MustCompile(`\{[\w.]*\}`)
var markupTagPattern = regexp.MustCompile(`(?s)<[^<>]*>`)
var markupSpacePattern = regexp.MustCompile(`\s+`)
var markupIconPattern = regexp.MustCompile(`^\{\{-?\s*icon\s+(?:"([\w-]+)"|\$[\w.]+)(?:\s+"([^"]*)")?`)
var markupCallPattern = regexp.MustCompile(`^\{\{-?\s*(?:template|block)\s+"([^"]*)"`)
var markupBranchPattern = regexp.MustCompile(`\{\{-?\s*(if|with|range|block|define|else|end)\b[^}]*\}\}`)
var markupSvgPattern = regexp.MustCompile(`(?is)<svg\b.*?</svg>`)
var markupNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:(?:--|__|-)[a-z0-9]+)*$`)

const markupDynamic = "\x00"

type Finding struct {
	Line int
	Text string
}

type Attribute struct {
	Name  string
	Value string
}

type Element struct {
	Name             string
	Attributes       []Attribute
	Classes          []string
	DynamicClass     bool
	ConditionalClass bool
	Offset           int
	Line             int
	ContentStart     int
	ContentEnd       int
	End              int
	Parent           *Element
	Children         []*Element
	Conditional      bool
	Partial          bool
	Opaque           bool
	Branches         map[int]int
}

type Markup struct {
	Text     string
	Elements []*Element
	Tree     bool
	Calls    []string
}

type frame struct {
	kind   string
	id     int
	branch int
	stack  []*Element
}

func WalkFiles(root string, exclusions []string, extensions map[string]bool, visit func(relative, extension, text string)) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && path != root || entry.Name() == "node_modules" || entry.Name() == "vendor" || IsExcluded(relative, exclusions) {
				return filepath.SkipDir
			}
			return nil
		}
		extension := filepath.Ext(path)
		if !extensions[extension] || IsExcluded(relative, exclusions) || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(relative, extension, string(content))
		return nil
	})
}

func BlankMatches(text string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(text, func(hidden string) string {
		return strings.Map(func(character rune) rune {
			if character == '\n' {
				return character
			}
			return ' '
		}, hidden)
	})
}

func ReportFindings(relative string, findings []Finding) []string {
	sort.SliceStable(findings, func(left, right int) bool { return findings[left].Line < findings[right].Line })
	var problems []string
	for _, finding := range findings {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", relative, finding.Line, finding.Text))
	}
	return problems
}

func ScanTag(text string, start int) (int, int, bool) {
	index := start + 1
	for index < len(text) && (text[index] >= 'a' && text[index] <= 'z' || text[index] >= 'A' && text[index] <= 'Z' || index > start+1 && (text[index] >= '0' && text[index] <= '9' || text[index] == '-')) {
		index++
	}
	if index == start+1 || index >= len(text) || !strings.ContainsRune(" \t\r\n/>", rune(text[index])) {
		return 0, 0, false
	}
	nameEnd := index
	var quote byte
	previous := byte(' ')
	for ; index < len(text) && index < start+6000; index++ {
		character := text[index]
		switch {
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case strings.HasPrefix(text[index:], "{{"):
			close := strings.Index(text[index:], "}}")
			if close < 0 {
				return 0, 0, false
			}
			index += close + 1
		case strings.HasPrefix(text[index:], "${"):
			close := strings.IndexByte(text[index:], '}')
			if close < 0 {
				return 0, 0, false
			}
			index += close
		case (character == '"' || character == '\'') && previous == '=':
			quote = character
		case character == '<':
			return 0, 0, false
		case character == '>':
			return nameEnd, index + 1, true
		}
		if character != ' ' && character != '\t' && character != '\n' && character != '\r' {
			previous = character
		}
	}
	return 0, 0, false
}

func ParseAttributes(raw, extension string) []Attribute {
	raw = markupValuePattern.ReplaceAllStringFunc(raw, func(action string) string {
		if markupActionPattern.MatchString(action) {
			return " "
		}
		return markupDynamic
	})
	if extension == ".py" {
		raw = markupFormatPattern.ReplaceAllString(raw, markupDynamic)
	}
	if extension == ".py" || extension == ".go" {
		raw = markupVerbPattern.ReplaceAllString(raw, markupDynamic)
	}
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "/")
	var attributes []Attribute
	for index := 0; index < len(raw); {
		for index < len(raw) && strings.ContainsRune(" \t\r\n/", rune(raw[index])) {
			index++
		}
		start := index
		for index < len(raw) && !strings.ContainsRune(" \t\r\n=/\"'", rune(raw[index])) {
			index++
		}
		if index == start {
			index++
			continue
		}
		attribute := Attribute{Name: raw[start:index]}
		rest := strings.TrimLeft(raw[index:], " \t\r\n")
		if strings.HasPrefix(rest, "=") {
			rest = strings.TrimLeft(rest[1:], " \t\r\n")
			index = len(raw) - len(rest)
			if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
				end := strings.IndexByte(rest[1:], rest[0])
				if end < 0 {
					end = len(rest) - 1
				}
				attribute.Value = rest[1 : 1+end]
				index += end + 2
			} else {
				end := strings.IndexAny(rest, " \t\r\n")
				if end < 0 {
					end = len(rest)
				}
				attribute.Value = rest[:end]
				index += end
			}
		}
		attributes = append(attributes, attribute)
	}
	return attributes
}

func ParseMarkup(text, extension string) *Markup {
	tree := templateExtensions[extension]
	if tree {
		text = BlankMatches(text, markupHiddenPattern)
	}
	markup := &Markup{Text: text, Tree: tree}
	var stack []*Element
	var frames []*frame
	conditional, partial, nextFrame := 0, 0, 0
	closeTo := func(depth, offset int) {
		for _, open := range stack[depth:] {
			if open.ContentEnd == 0 {
				open.ContentEnd = offset
				open.End = offset
			}
		}
		stack = stack[:depth]
	}
	register := func(element *Element) {
		element.Line = LineOf(text, element.Offset)
		element.Conditional = conditional > 0
		element.Partial = partial > 0
		if tree {
			if len(stack) > 0 {
				element.Parent = stack[len(stack)-1]
				element.Parent.Children = append(element.Parent.Children, element)
			}
			element.Branches = map[int]int{}
			for _, top := range frames {
				if top.kind != "define" && top.kind != "block" {
					element.Branches[top.id] = top.branch
				}
			}
		}
		markup.Elements = append(markup.Elements, element)
	}
	for index := 0; index < len(text); {
		switch {
		case strings.HasPrefix(text[index:], "{{"):
			window := text[index:min(index+160, len(text))]
			if found := markupIconPattern.FindStringSubmatch(window); found != nil && tree {
				glyph := found[1]
				if glyph == "" {
					glyph = markupDynamic
				}
				icon := &Element{Name: "svg", Offset: index, ContentStart: index, ContentEnd: index + 1, End: index + 1, Classes: []string{"icon"}}
				for _, class := range strings.Fields(found[2]) {
					if markupNamePattern.MatchString(class) {
						icon.Classes = append(icon.Classes, class)
					}
				}
				register(icon)
				use := &Element{Name: "use", Offset: index, ContentStart: index, ContentEnd: index + 1, End: index + 1, Attributes: []Attribute{{"href", "/icons.svg#" + glyph}}}
				register(use)
				use.Parent = icon
				icon.Children = append(icon.Children, use)
				if len(stack) > 0 {
					stack[len(stack)-1].Children = stack[len(stack)-1].Children[:len(stack[len(stack)-1].Children)-1]
					stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, icon)
				}
			}
			if found := markupActionPattern.FindStringSubmatch(window); found != nil && tree {
				if name := markupCallPattern.FindStringSubmatch(window); name != nil {
					markup.Calls = append(markup.Calls, name[1])
				}
				switch word := found[1]; word {
				case "template":
					for _, open := range stack {
						open.Opaque = true
					}
				case "else":
					if len(frames) > 0 && frames[len(frames)-1].kind != "define" {
						top := frames[len(frames)-1]
						top.branch++
						if len(stack) > len(top.stack) {
							closeTo(len(top.stack), index)
						}
						stack = append([]*Element(nil), top.stack...)
					}
				case "end":
					if len(frames) > 0 {
						top := frames[len(frames)-1]
						frames = frames[:len(frames)-1]
						switch top.kind {
						case "define":
							stack = top.stack
							partial--
						case "block":
						default:
							conditional--
						}
					}
				default:
					top := &frame{kind: word, id: nextFrame, stack: append([]*Element(nil), stack...)}
					nextFrame++
					frames = append(frames, top)
					switch word {
					case "define":
						stack = nil
						partial++
					case "block":
						for _, open := range stack {
							open.Opaque = true
						}
					default:
						conditional++
					}
				}
			}
			index += 2
		case text[index] == '<' && index+1 < len(text) && text[index+1] == '/':
			end := strings.IndexByte(text[index:], '>')
			if end < 0 {
				return markup
			}
			name := strings.ToLower(strings.TrimSpace(text[index+2 : index+end]))
			for depth := len(stack) - 1; tree && depth >= 0; depth-- {
				if stack[depth].Name == name {
					stack[depth].ContentEnd = index
					stack[depth].End = index + end + 1
					closeTo(depth, index)
					break
				}
			}
			index += end + 1
		case text[index] == '<':
			nameEnd, end, ok := ScanTag(text, index)
			if !ok {
				index++
				continue
			}
			element := &Element{Name: strings.ToLower(text[index+1 : nameEnd]), Offset: index, ContentStart: end}
			element.Attributes = ParseAttributes(text[nameEnd:end-1], extension)
			for _, attribute := range element.Attributes {
				if strings.EqualFold(attribute.Name, "class") || strings.EqualFold(attribute.Name, "classname") {
					element.ConditionalClass = element.ConditionalClass || strings.Contains(text[index:end], "{{if") || strings.Contains(text[index:end], "{{ if") || strings.Contains(text[index:end], "{{else")
					for _, token := range strings.Fields(attribute.Value) {
						switch {
						case strings.Contains(token, markupDynamic):
							element.DynamicClass = true
						case markupNamePattern.MatchString(token):
							element.Classes = append(element.Classes, token)
						case strings.HasSuffix(token, "-") || strings.HasSuffix(token, "_"):
							element.DynamicClass = true
						}
					}
				}
			}
			register(element)
			selfClosing := strings.HasSuffix(strings.TrimSpace(text[nameEnd:end-1]), "/")
			if tree && !selfClosing && !voidElements[element.Name] {
				stack = append(stack, element)
			} else {
				element.ContentEnd = end
				element.End = end
			}
			index = end
		default:
			next := strings.IndexAny(text[index+1:], "<{")
			if next < 0 {
				index = len(text)
			} else {
				index += next + 1
			}
		}
	}
	closeTo(0, len(text))
	for _, element := range markup.Elements {
		if element.ContentEnd == 0 {
			element.ContentEnd = len(text)
			element.End = len(text)
		}
	}
	return markup
}

func (element *Element) Attribute(name string) (string, bool) {
	for _, attribute := range element.Attributes {
		if strings.EqualFold(attribute.Name, name) {
			return attribute.Value, true
		}
	}
	return "", false
}

func (element *Element) Has(class string) bool {
	for _, candidate := range element.Classes {
		if candidate == class {
			return true
		}
	}
	return false
}

func (element *Element) Descendants() []*Element {
	var found []*Element
	for _, child := range element.Children {
		found = append(found, child)
		found = append(found, child.Descendants()...)
	}
	return found
}

func (element *Element) Within(selector ElementSelector) bool {
	for parent := element.Parent; parent != nil; parent = parent.Parent {
		if selector.Matches(parent) {
			return true
		}
	}
	return false
}

func (element *Element) IsWhole() bool {
	for current := element; current != nil; current = current.Parent {
		if current.Partial || current.DynamicClass {
			return false
		}
		if current.Parent == nil {
			return current.Name == "html" || current.Name == "body"
		}
	}
	return false
}

func (element *Element) ExclusiveWith(other *Element) bool {
	for id, branch := range element.Branches {
		if otherBranch, shared := other.Branches[id]; shared && otherBranch != branch {
			return true
		}
	}
	return false
}

func (markup *Markup) ReadText(element *Element) string {
	return strings.Join(markup.ReadTexts(element), " ")
}

func (markup *Markup) ReadTexts(element *Element) []string {
	return ReadVariants(markup.Text[element.ContentStart:element.ContentEnd])
}

func (markup *Markup) ReadLabels(element *Element) []string {
	content := []byte(markup.Text[element.ContentStart:element.ContentEnd])
	for _, child := range element.Children {
		for _, class := range child.Classes {
			if strings.HasSuffix(class, "__desc") || strings.HasSuffix(class, "__meta") || strings.HasSuffix(class, "__value") || strings.HasSuffix(class, "__count") || class == "badge" || strings.HasSuffix(class, "__badge") {
				for index := child.Offset - element.ContentStart; index < child.End-element.ContentStart && index < len(content); index++ {
					if content[index] != '\n' {
						content[index] = ' '
					}
				}
			}
		}
	}
	return ReadVariants(string(content))
}

func HasText(texts []string, want string) bool {
	for _, text := range texts {
		if text == want {
			return true
		}
	}
	return false
}

func ReadVariants(content string) []string {
	type branch struct {
		kind     string
		before   []string
		finished []string
		split    bool
	}
	content = BlankMatches(content, markupSvgPattern)
	content = markupTagPattern.ReplaceAllString(content, " ")
	current := []string{""}
	var branches []*branch
	extend := func(text string) {
		text = html.UnescapeString(markupValuePattern.ReplaceAllString(text, markupDynamic))
		for index := range current {
			current[index] += text
		}
	}
	position := 0
	for _, at := range markupBranchPattern.FindAllStringSubmatchIndex(content, -1) {
		extend(content[position:at[0]])
		position = at[1]
		switch word := content[at[2]:at[3]]; word {
		case "else":
			if len(branches) > 0 {
				top := branches[len(branches)-1]
				top.finished = append(top.finished, current...)
				top.split = true
				current = []string{""}
			}
		case "end":
			if len(branches) > 0 {
				top := branches[len(branches)-1]
				branches = branches[:len(branches)-1]
				alternatives := append(top.finished, current...)
				if (top.kind == "if" || top.kind == "with") && !top.split {
					alternatives = append(alternatives, "")
				}
				current = nil
				for _, before := range top.before {
					for _, alternative := range alternatives {
						if len(current) < 64 {
							current = append(current, before+alternative)
						}
					}
				}
			}
		default:
			branches = append(branches, &branch{kind: word, before: current})
			current = []string{""}
		}
	}
	extend(content[position:])
	var texts []string
	for _, text := range Deduplicate(current) {
		texts = append(texts, strings.TrimSpace(markupSpacePattern.ReplaceAllString(text, " ")))
	}
	return Deduplicate(texts)
}

type ElementSelector struct {
	Tags       []string
	Classes    []string
	Attributes []AttributeMatch
}

type AttributeMatch struct {
	Name     string
	Operator string
	Value    string
}

var selectorPattern = regexp.MustCompile(`^([a-z0-9|*-]*)((?:\.[\w-]+)*)((?:\[[^\]]+\])*)$`)
var selectorAttributePattern = regexp.MustCompile(`\[([\w-]+)(?:(\$?=)([^\]]*))?\]`)

func ParseSelector(spec string) ElementSelector {
	found := selectorPattern.FindStringSubmatch(spec)
	if found == nil {
		panic("bad selector " + spec)
	}
	selector := ElementSelector{}
	if found[1] != "" && found[1] != "*" {
		selector.Tags = strings.Split(found[1], "|")
	}
	for _, class := range strings.Split(found[2], ".") {
		if class != "" {
			selector.Classes = append(selector.Classes, class)
		}
	}
	for _, attribute := range selectorAttributePattern.FindAllStringSubmatch(found[3], -1) {
		selector.Attributes = append(selector.Attributes, AttributeMatch{attribute[1], attribute[2], attribute[3]})
	}
	return selector
}

func (selector ElementSelector) Matches(element *Element) bool {
	if len(selector.Tags) > 0 {
		matched := false
		for _, tag := range selector.Tags {
			matched = matched || tag == element.Name
		}
		if !matched {
			return false
		}
	}
	for _, class := range selector.Classes {
		if !element.Has(class) {
			return false
		}
	}
	for _, match := range selector.Attributes {
		value, present := element.Attribute(match.Name)
		switch {
		case !present:
			return false
		case match.Operator == "=" && value != match.Value:
			return false
		case match.Operator == "$=" && !strings.HasSuffix(value, match.Value):
			return false
		}
	}
	return true
}

func (selector ElementSelector) Find(element *Element) []*Element {
	var found []*Element
	for _, descendant := range element.Descendants() {
		if selector.Matches(descendant) {
			found = append(found, descendant)
		}
	}
	return found
}

func HasDescendant(element *Element, specs string) bool {
	for _, spec := range strings.Split(specs, " or ") {
		if len(ParseSelector(spec).Find(element)) > 0 {
			return true
		}
	}
	return false
}
