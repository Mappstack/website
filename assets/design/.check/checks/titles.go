package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var titleSmallWords = map[string]bool{"a": true, "an": true, "the": true, "and": true, "but": true, "or": true, "for": true, "nor": true, "to": true, "as": true, "of": true, "in": true, "on": true, "at": true, "by": true, "up": true, "off": true, "out": true, "via": true, "per": true, "with": true, "from": true, "into": true, "over": true, "onto": true, "near": true, "upon": true, "than": true, "vs": true}
var titleSelectors = func() []ElementSelector {
	var selectors []ElementSelector
	for _, spec := range []string{"button", ".btn", ".menu-item", "[role=tab]", ".tabbar__tab", ".sidebar-item__label", ".field__label", ".dialog__title", ".outcome__title", ".empty__title", ".notice__title", ".page-title", ".doc__title", ".auth__title", ".navbar__title"} {
		selectors = append(selectors, ParseSelector(spec))
	}
	return selectors
}()
var punctuatedSelectors = []ElementSelector{ParseSelector(".outcome__title"), ParseSelector(".notice__title")}
var sentenceBreakPattern = regexp.MustCompile(`[.!?]\s+[A-Z]`)
var lastUpdatedFormat = regexp.MustCompile(`^(?:\s|&nbsp;)*(?:\x00|(?:January|February|March|April|May|June|July|August|September|October|November|December) \d{1,2}, \d{4})`)
var lastUpdatedPhrase = regexp.MustCompile(`(?i)last\s+updated`)

func FindTitleProblems(markup *Markup) []Finding {
	var findings []Finding
	for _, element := range markup.Elements {
		if element.Has("btn--icon") {
			continue
		}
		for _, label := range markup.ReadLabels(element) {
			if label == "" {
				continue
			}
			printable := strings.ReplaceAll(label, markupDynamic, "…")
			if IsTitled(element) && !(strings.HasSuffix(label, ".") && (element.Has("page-title") || element.Has("doc__title") || element.Has("auth__title"))) && !(element.Has("field__label") && (strings.HasSuffix(label, "?") || len(strings.Fields(label)) > 4)) {
				if word := TitleCaseSlip(label); word != "" {
					findings = append(findings, Finding{element.Line, fmt.Sprintf("title case: %q in %q: Copy: Title Case for buttons, titles and menu items", word, printable)})
				}
			}
			for _, selector := range punctuatedSelectors {
				if selector.Matches(element) && strings.ContainsAny(label[len(label)-1:], ".!:,;") {
					findings = append(findings, Finding{element.Line, fmt.Sprintf("punctuation after %q: Copy: alert and status titles are short phrases with no punctuation", printable)})
					break
				}
			}
			if element.Has("empty__desc") && sentenceBreakPattern.MatchString(label) {
				findings = append(findings, Finding{element.Line, fmt.Sprintf("more than one sentence in %q: Copy: an empty state is a title and one sentence", printable)})
			}
		}
	}
	return findings
}

func IsTitled(element *Element) bool {
	for _, selector := range titleSelectors {
		if selector.Matches(element) {
			return true
		}
	}
	return false
}

func TitleCaseSlip(label string) string {
	if strings.Contains(label, markupDynamic) {
		return ""
	}
	words := strings.Fields(label)
	for index, word := range words {
		first := []rune(word)[0]
		if strings.ContainsAny(word, markupDynamic+"@./:_") || !unicode.IsLetter(first) || word != strings.ToLower(word) && word[1:] != strings.ToLower(word[1:]) {
			continue
		}
		last := index == len(words)-1
		switch {
		case unicode.IsUpper(first):
		case index > 0 && !last && titleSmallWords[strings.Trim(word, ",;:")]:
		default:
			return word
		}
	}
	return ""
}

func FindDateProblems(markup *Markup) []Finding {
	var findings []Finding
	for _, element := range markup.Elements {
		if !element.Has("doc__meta") && !element.Has("prose__meta") {
			continue
		}
		for _, text := range markup.ReadTexts(element) {
			for _, match := range lastUpdatedPhrase.FindAllStringIndex(text, -1) {
				if text[match[0]:match[1]] != "Last updated" || !lastUpdatedFormat.MatchString(text[match[1]:]) {
					findings = append(findings, Finding{element.Line, fmt.Sprintf("date line %q: Long-form: \"Last updated <Month D, YYYY>\"", strings.ReplaceAll(text, markupDynamic, "…"))})
				}
			}
		}
	}
	return findings
}
