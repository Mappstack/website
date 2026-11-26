package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	matchNo = iota
	matchMaybe
	matchYes
)

var attributeSelectorPattern = regexp.MustCompile(`^\[\s*([\w-]+)\s*(?:([~|^$*]?=)\s*("[^"]*"|'[^']*'|[^\s\]]*))?\s*(?:[is])?\s*\]$`)
var selectorListPseudos = map[string]bool{"is": true, "where": true, "not": true, "has": true}
var pseudoElementNames = map[string]bool{"before": true, "after": true, "first-line": true, "first-letter": true}
var openingForms = map[string]string{"(min-width:900px)": "min900", "(width>=900px)": "min900", "(max-width:899px)": "max899", "(max-width:899.98px)": "max899", "(max-width:899.9px)": "max899", "(width<900px)": "max899"}

type simpleSelector struct {
	kind  byte
	name  string
	op    string
	value string
	args  []complexSelector
	loose bool
}

type compoundSelector struct {
	simples []simpleSelector
	joint   byte
}

type complexSelector struct {
	parts       []compoundSelector
	specificity int
}

type drawnEntry struct {
	line, offset, size, specificity int
	query                           bool
	key                             string
}

type drawnStep struct {
	context          string
	opening, reverse bool
	from, to         int
	keys             []string
}

type drawnSize struct {
	size int
	key  string
}

func DeclarationKey(rule StyleRule, declaration StyleDeclaration) string {
	return rule.Source + "\x00" + rule.Selector + "\x00" + strings.Join(rule.Contexts, "\n") + "\x00" + strconv.Itoa(declaration.Line)
}

func readName(text string, index int) (string, int) {
	var name []byte
	for index < len(text) && strings.IndexByte(".#[:(&", text[index]) < 0 {
		if text[index] == '\\' && index+1 < len(text) {
			index++
		}
		name = append(name, text[index])
		index++
	}
	return string(name), index
}

func readBalanced(text string, index int) int {
	depth := 0
	var quote byte
	for ; index < len(text); index++ {
		character := text[index]
		switch {
		case quote != 0:
			if character == '\\' {
				index++
			} else if character == quote {
				quote = 0
			}
		case character == '"' || character == '\'':
			quote = character
		case character == '(' || character == '[':
			depth++
		case character == ')' || character == ']':
			depth--
			if depth == 0 {
				return index + 1
			}
		}
	}
	return -1
}

func parseSelectorList(text string) ([]complexSelector, bool) {
	var list []complexSelector
	for _, part := range SplitTopLevel(text) {
		selector, ok := parseComplex(part)
		if !ok {
			return nil, false
		}
		list = append(list, selector)
	}
	return list, true
}

func parseCompound(text string) (compoundSelector, bool) {
	var compound compoundSelector
	index := 0
	if text != "" && strings.IndexByte(".#[:&", text[0]) < 0 {
		name, next := readName(text, 0)
		compound.simples = append(compound.simples, simpleSelector{kind: 't', name: strings.ToLower(name)})
		index = next
	}
	for index < len(text) {
		switch text[index] {
		case '.', '#':
			name, next := readName(text, index+1)
			kind := byte('c')
			if text[index] == '#' {
				kind = 'i'
			}
			compound.simples = append(compound.simples, simpleSelector{kind: kind, name: name})
			index = next
		case '[':
			end := readBalanced(text, index)
			if end < 0 {
				return compound, false
			}
			simple := simpleSelector{kind: 'a', loose: true}
			if found := attributeSelectorPattern.FindStringSubmatch(text[index:end]); found != nil {
				simple = simpleSelector{kind: 'a', name: found[1], op: found[2], value: strings.Trim(found[3], `"'`)}
			}
			compound.simples = append(compound.simples, simple)
			index = end
		case ':':
			colons := 1
			if index+1 < len(text) && text[index+1] == ':' {
				colons = 2
			}
			name, next := readName(text, index+colons)
			simple := simpleSelector{kind: 'p', name: strings.ToLower(name)}
			if colons == 2 || pseudoElementNames[simple.name] {
				simple.kind = 'e'
			}
			if next < len(text) && text[next] == '(' {
				end := readBalanced(text, next)
				if end < 0 {
					return compound, false
				}
				if selectorListPseudos[simple.name] {
					if list, ok := parseSelectorList(text[next+1 : end-1]); ok {
						simple.args = list
					} else {
						simple.loose = true
					}
				}
				next = end
			}
			compound.simples = append(compound.simples, simple)
			index = next
		case '&':
			compound.simples = append(compound.simples, simpleSelector{kind: '&'})
			index++
		default:
			return compound, false
		}
	}
	return compound, len(compound.simples) > 0
}

func parseComplex(text string) (complexSelector, bool) {
	var selector complexSelector
	text = strings.TrimSpace(text)
	for index := 0; index < len(text); {
		joint := byte(0)
		for index < len(text) && strings.IndexByte(" \t\r\n>+~", text[index]) >= 0 {
			if strings.IndexByte(">+~", text[index]) >= 0 {
				joint = text[index]
			} else if joint == 0 {
				joint = ' '
			}
			index++
		}
		if index >= len(text) || len(selector.parts) == 0 && joint != 0 {
			return selector, false
		}
		start := index
		depth := 0
		var quote byte
		for ; index < len(text); index++ {
			character := text[index]
			if quote != 0 {
				if character == '\\' {
					index++
				} else if character == quote {
					quote = 0
				}
				continue
			}
			if character == '\\' {
				index++
			} else if character == '"' || character == '\'' {
				quote = character
			} else if character == '(' || character == '[' {
				depth++
			} else if character == ')' || character == ']' {
				depth--
			} else if depth == 0 && strings.IndexByte(" \t\r\n>+~", character) >= 0 {
				break
			}
		}
		compound, ok := parseCompound(text[start:index])
		if !ok {
			return selector, false
		}
		compound.joint = joint
		selector.parts = append(selector.parts, compound)
	}
	for _, part := range selector.parts {
		selector.specificity += ReadSpecificity(part)
	}
	return selector, len(selector.parts) > 0
}

func ReadSpecificity(compound compoundSelector) int {
	total := 0
	for _, simple := range compound.simples {
		switch simple.kind {
		case 'i':
			total += 10000
		case 'c', 'a':
			total += 100
		case 't':
			if simple.name != "*" {
				total++
			}
		case 'e':
			total++
		case 'p':
			if simple.name == "where" {
				continue
			}
			if len(simple.args) == 0 {
				total += 100
				continue
			}
			most := 0
			for _, argument := range simple.args {
				most = max(most, argument.specificity)
			}
			total += most
		}
	}
	return total
}

func matchSimple(simple simpleSelector, element *Element, tree bool) int {
	switch simple.kind {
	case 't':
		if simple.name == "*" || simple.name == element.Name {
			return matchYes
		}
		return matchNo
	case 'c':
		switch {
		case !element.Has(simple.name):
			return matchNo
		case element.BranchClasses[simple.name]:
			return matchMaybe
		}
		return matchYes
	case 'i', 'a':
		name, operator, expected := simple.name, simple.op, simple.value
		if simple.kind == 'i' {
			name, operator, expected = "id", "=", simple.name
		}
		if simple.loose {
			return matchMaybe
		}
		value, present := element.Attribute(name)
		switch {
		case !present:
			return matchNo
		case strings.Contains(value, markupDynamic) || strings.Contains(value, "{{"):
			return matchMaybe
		}
		holds := true
		switch operator {
		case "=":
			holds = value == expected
		case "~=":
			holds = false
			for _, word := range strings.Fields(value) {
				holds = holds || word == expected
			}
		case "|=":
			holds = value == expected || strings.HasPrefix(value, expected+"-")
		case "^=":
			holds = strings.HasPrefix(value, expected)
		case "$=":
			holds = strings.HasSuffix(value, expected)
		case "*=":
			holds = strings.Contains(value, expected)
		}
		if holds {
			return matchYes
		}
		return matchNo
	case 'e':
		return matchNo
	case '&':
		return matchMaybe
	}
	switch simple.name {
	case "root":
		if element.Name == "html" {
			return matchYes
		}
		return matchNo
	case "is", "where", "not":
		if simple.loose || len(simple.args) == 0 {
			return matchMaybe
		}
		best := matchNo
		for _, argument := range simple.args {
			best = max(best, matchComplex(argument, len(argument.parts)-1, element, tree))
		}
		if simple.name == "not" {
			return matchYes - best
		}
		return best
	}
	return matchMaybe
}

func matchCompound(compound compoundSelector, element *Element, tree bool) int {
	result := matchYes
	for _, simple := range compound.simples {
		result = min(result, matchSimple(simple, element, tree))
		if result == matchNo {
			return matchNo
		}
	}
	return result
}

func ReadUnknownAncestors(top *Element, tree bool) int {
	if !tree || top.Partial {
		return matchMaybe
	}
	return matchNo
}

func matchComplex(selector complexSelector, index int, element *Element, tree bool) int {
	part := selector.parts[index]
	result := matchCompound(part, element, tree)
	if result == matchNo || index == 0 {
		return result
	}
	switch part.joint {
	case '>':
		if element.Parent == nil {
			return min(result, ReadUnknownAncestors(element, tree))
		}
		return min(result, matchComplex(selector, index-1, element.Parent, tree))
	case ' ':
		best, top := matchNo, element
		for ancestor := element.Parent; ancestor != nil && best != matchYes; ancestor = ancestor.Parent {
			best = max(best, matchComplex(selector, index-1, ancestor, tree))
			top = ancestor
		}
		if best != matchYes {
			best = max(best, ReadUnknownAncestors(top, tree))
		}
		return min(result, best)
	}
	if !tree {
		return min(result, matchMaybe)
	}
	if element.Parent == nil {
		return min(result, ReadUnknownAncestors(element, tree))
	}
	best, later := matchNo, false
	siblings := element.Parent.Children
	for position := len(siblings) - 1; position >= 0; position-- {
		if siblings[position] == element {
			later = true
			continue
		}
		if !later {
			continue
		}
		found := matchComplex(selector, index-1, siblings[position], tree)
		if siblings[position].Conditional || element.Conditional {
			found = min(found, matchMaybe)
		}
		best = max(best, found)
		if part.joint == '+' {
			break
		}
	}
	return min(result, best)
}

func ReadOpeningForm(prelude string) string {
	form := ""
	for _, alternative := range SplitTopLevel(strings.ToLower(prelude)) {
		if IsPrintAlternative(alternative) {
			continue
		}
		alternative = strings.Join(strings.Fields(alternative), "")
		for _, lead := range []string{"onlyscreenand", "screenand", "alland"} {
			alternative = strings.TrimPrefix(alternative, lead)
		}
		found := openingForms[alternative]
		if found == "" || form != "" && form != found {
			return ""
		}
		form = found
	}
	return form
}

func ReadQueryKey(rule StyleRule) (string, bool) {
	var others []string
	var sizes []string
	for _, context := range rule.Contexts {
		prelude, isMedia := strings.CutPrefix(context, "@media")
		switch {
		case strings.HasPrefix(context, "@container"):
			sizes = append(sizes, "")
		case isMedia && IsSizeMediaQuery(prelude):
			sizes = append(sizes, ReadOpeningForm(prelude))
		default:
			others = append(others, context)
		}
	}
	rest := strings.Join(others, "\n")
	if len(sizes) == 1 && sizes[0] != "" {
		return sizes[0] + "|" + rest, true
	}
	return strings.Join(rule.Contexts, "\n"), false
}

func ReadComplement(key string) string {
	switch {
	case strings.HasPrefix(key, "min900|"):
		return "max899|" + strings.TrimPrefix(key, "min900|")
	case strings.HasPrefix(key, "max899|"):
		return "min900|" + strings.TrimPrefix(key, "max899|")
	}
	return ""
}

func JudgeSubject(bases map[int]bool, queries map[string][]drawnSize, verdicts map[string]string) {
	if len(bases) == 0 {
		bases = map[int]bool{-1: true}
	}
	var steps []drawnStep
	paired := map[string]bool{}
	for key, list := range queries {
		opening := strings.HasPrefix(key, "min900|") || strings.HasPrefix(key, "max899|")
		other, both := queries[ReadComplement(key)]
		if both {
			if paired[key] {
				continue
			}
			paired[key], paired[ReadComplement(key)] = true, true
			for _, first := range list {
				for _, second := range other {
					if first.size != second.size {
						steps = append(steps, drawnStep{"900|" + key[strings.IndexByte(key, '|')+1:], true, first.size > second.size == strings.HasPrefix(key, "max899|"), first.size, second.size, []string{first.key, second.key}})
					}
				}
			}
			continue
		}
		for _, drawn := range list {
			for from := range bases {
				if from != drawn.size {
					steps = append(steps, drawnStep{key, opening, from >= 0 && drawn.size > from == strings.HasPrefix(key, "max899|"), from, drawn.size, []string{drawn.key}})
				}
			}
		}
	}
	display := map[string]bool{}
	for _, step := range steps {
		if step.from < 0 && step.to < 40 || step.from >= 0 && max(step.from, step.to) < 40 {
			for _, key := range step.keys {
				if verdicts[key] == "" {
					verdicts[key] = "changes with the viewport"
				}
			}
		} else {
			display[step.context] = true
		}
	}
	for _, step := range steps {
		if step.from < 0 && step.to < 40 || step.from >= 0 && max(step.from, step.to) < 40 {
			continue
		}
		for _, key := range step.keys {
			switch {
			case verdicts[key] != "":
			case len(display) > 1:
				verdicts[key] = "steps display type more than once"
			case !step.opening:
				verdicts[key] = "steps display type at a width other than 900"
			case step.reverse:
				verdicts[key] = "draws display type bigger below 900"
			}
		}
	}
}

func JudgeSteps(rules []StyleRule, pages []*Markup, model *TypeModel) {
	type alternative struct {
		selector complexSelector
		ok       bool
	}
	parsed := map[int][]alternative{}
	shown := map[[2]int]bool{}
	var sized []int
	for index, rule := range rules {
		if IsPrintRule(rule) || strings.HasPrefix(rule.Selector, "style ") {
			continue
		}
		for _, declaration := range rule.Declarations {
			if model.Size[declaration.Property] {
				sized = append(sized, index)
				for _, text := range SplitTopLevel(rule.Selector) {
					selector, ok := parseComplex(text)
					parsed[index] = append(parsed[index], alternative{selector, ok})
				}
				break
			}
		}
	}
	for _, page := range pages {
		for _, element := range page.Elements {
			classes, branches := element.Classes, element.BranchClasses
			variants := element.ClassVariants
			if len(variants) == 0 {
				variants = [][]string{classes}
			}
			for _, variant := range variants {
				element.Classes, element.BranchClasses = variant, nil
				for _, strict := range []bool{false, true} {
					type group struct {
						ground  []drawnEntry
						plain   map[string][]drawnEntry
						queries map[string][]drawnEntry
					}
					properties := map[string]map[string]*group{}
					for _, index := range sized {
						rule := rules[index]
						specificity, matched := 0, false
						for position, candidate := range parsed[index] {
							if !candidate.ok {
								continue
							}
							if level := matchComplex(candidate.selector, len(candidate.selector.parts)-1, element, page.Tree); level != matchNo && (!strict || level == matchYes) {
								matched = true
								specificity = max(specificity, candidate.selector.specificity)
								shown[[2]int{index, position}] = true
							}
						}
						if !matched {
							continue
						}
						key, _ := ReadQueryKey(rule)
						for _, declaration := range rule.Declarations {
							if !model.Size[declaration.Property] {
								continue
							}
							name := NameSizeProperty(declaration.Property)
							if properties[name] == nil {
								properties[name] = map[string]*group{}
							}
							if properties[name][rule.Source] == nil {
								properties[name][rule.Source] = &group{plain: map[string][]drawnEntry{}, queries: map[string][]drawnEntry{}}
							}
							present := properties[name][rule.Source]
							size, bare := ReadBareSize(declaration.Property, declaration.Value)
							if !bare {
								size = -1
							}
							entry := drawnEntry{declaration.Line, declaration.Offset, size, specificity, IsSizeQuery(rule), DeclarationKey(rule, declaration)}
							switch {
							case entry.query:
								present.queries[key] = append(present.queries[key], entry)
							case len(rule.Contexts) == 0:
								present.ground = append(present.ground, entry)
							default:
								present.plain[strings.Join(rule.Contexts, "\n")] = append(present.plain[strings.Join(rule.Contexts, "\n")], entry)
							}
						}
					}
					winner := func(lists ...[]drawnEntry) (drawnEntry, bool) {
						var best drawnEntry
						found := false
						for _, list := range lists {
							for _, entry := range list {
								if !found || entry.specificity > best.specificity || entry.specificity == best.specificity && (entry.line > best.line || entry.line == best.line && entry.offset > best.offset) {
									best, found = entry, true
								}
							}
						}
						return best, found
					}
					for _, sources := range properties {
						bases := map[int]bool{}
						queries := map[string][]drawnSize{}
						for _, present := range sources {
							if best, found := winner(present.ground); found {
								bases[best.size] = true
							}
							for _, list := range present.plain {
								if best, found := winner(present.ground, list); found {
									bases[best.size] = true
								}
							}
						}
						for _, present := range sources {
							for key, list := range present.queries {
								best, _ := winner(present.ground, list)
								if best.size >= 0 && (best.query || ReadComplement(key) != "") {
									queries[key] = append(queries[key], drawnSize{best.size, best.key})
								}
							}
						}
						JudgeSubject(bases, queries, model.Verdict)
					}
				}
			}
			element.Classes, element.BranchClasses = classes, branches
		}
	}
	pairs := map[string]map[string][]drawnSize{}
	for index, rule := range rules {
		if IsPrintRule(rule) || !IsSizeQuery(rule) {
			continue
		}
		key, _ := ReadQueryKey(rule)
		for _, declaration := range rule.Declarations {
			size, bare := ReadBareSize(declaration.Property, declaration.Value)
			if !model.Size[declaration.Property] || !bare {
				continue
			}
			for position, text := range SplitTopLevel(rule.Selector) {
				if model.Size[declaration.Property] && shown[[2]int{index, position}] {
					continue
				}
				subject := NormalizeSelector(text) + "|" + NameSizeProperty(declaration.Property)
				if pairs[subject] == nil {
					pairs[subject] = map[string][]drawnSize{}
				}
				pairs[subject][key] = append(pairs[subject][key], drawnSize{size, DeclarationKey(rule, declaration)})
			}
		}
	}
	var subjects []string
	for subject := range pairs {
		subjects = append(subjects, subject)
	}
	sort.Strings(subjects)
	for _, subject := range subjects {
		JudgeSubject(model.Base[subject], pairs[subject], model.Verdict)
	}
}
