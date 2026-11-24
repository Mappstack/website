package main

import (
	"regexp"
	"strings"
)

var motionQueryPattern = regexp.MustCompile(`^@media\s+(?:only\s+)?(?:all\s+and\s+)?\(\s*prefers-reduced-motion\s*:\s*no-preference\s*\)(?:\s+and\s+\([^()]*\))*\s*$`)
var timelineSupportPattern = regexp.MustCompile(`^@supports\s+\(\s*animation-timeline\s*:\s*[\w-]+\([^()]*\)\s*\)\s*$`)
var innerParenthesesPattern = regexp.MustCompile(`\([^()]*\)`)
var timelinePropertyPattern = regexp.MustCompile(`(?i)(animation-timeline|animation-range|view-timeline|scroll-timeline)`)
var timelineNamePattern = regexp.MustCompile(`--[\w-]+`)
var varCallPattern = regexp.MustCompile(`var\((?:[^()]|\([^()]*\))*\)`)
var bareNumberPattern = regexp.MustCompile(`-?\d*\.?\d+`)

// The system's own named timeline: the bar's title, which has its own reduced motion fallback.
var systemTimelines = map[string]bool{"--page-title": true}

func FindTimelineProblems(rules []StyleRule) []Finding {
	var findings []Finding
	for _, rule := range rules {
		reduced, supported := false, false
		for _, context := range rule.Contexts {
			reduced = reduced || motionQueryPattern.MatchString(context)
			supported = supported || timelineSupportPattern.MatchString(context)
		}
		for _, declaration := range rule.Declarations {
			if declaration.Property == "animation-range-start" || declaration.Property == "animation-range-end" {
				if problem := FindRangeProblem(declaration.Value + " " + declaration.Value); problem != "" || !startsNamed(declaration.Value) {
					findings = append(findings, Finding{declaration.Line, declaration.Property + " without a range name: Principles 9 and 10: name the range, or it is read as another"})
				}
			}
			if declaration.Property == "animation-range" {
				if problem := FindRangeProblem(declaration.Value); problem != "" {
					findings = append(findings, Finding{declaration.Line, "animation-range " + problem + ": Principles 9 and 10: name the range on both ends, or the end is read as another range"})
				}
			}
			if !usesScrollTimeline(declaration) || reduced && supported {
				continue
			}
			findings = append(findings, Finding{declaration.Line, declaration.Property + " " + declaration.Value + " outside @media (prefers-reduced-motion:no-preference) and @supports (animation-timeline:view()) together: Principles 9 and 10: it exists only where supported and motion is not reduced"})
		}
	}
	return findings
}

func startsNamed(value string) bool {
	tokens := strings.Fields(value)
	if len(tokens) == 0 {
		return false
	}
	return tokens[0] == "normal" || isRangeName(tokens[0])
}

func isRangeName(token string) bool {
	for _, name := range []string{"entry", "exit", "cover", "contain"} {
		if token == name || token == name+"-crossing" {
			return true
		}
	}
	return false
}

func usesScrollTimeline(declaration StyleDeclaration) bool {
	value := declaration.Value
	switch declaration.Property {
	case "animation-timeline", "animation":
		if strings.Contains(value, "view(") {
			return true
		}
		if declaration.Property == "animation-timeline" {
			for _, name := range timelineNamePattern.FindAllString(value, -1) {
				if !systemTimelines[name] {
					return true
				}
			}
		}
	case "view-timeline", "view-timeline-name", "scroll-timeline", "scroll-timeline-name":
		for _, name := range timelineNamePattern.FindAllString(value, -1) {
			if !systemTimelines[name] {
				return true
			}
		}
	}
	return false
}

func FindRangeProblem(value string) string {
	for innerParenthesesPattern.MatchString(value) {
		value = innerParenthesesPattern.ReplaceAllString(value, "X")
	}
	tokens := strings.Fields(value)
	if len(tokens) == 0 || !isRangeName(tokens[0]) {
		return ""
	}
	rest := tokens[1:]
	if len(rest) > 0 && !isRangeName(rest[0]) {
		rest = rest[1:]
	}
	if len(rest) > 0 && !isRangeName(rest[0]) {
		return "ends without a range name"
	}
	return ""
}

// FindBareNumbers returns the literal numbers in a value that carry no token, inside a
// calc too: what a closed property such as scale must take from a token. Zero, and one
// for scale, are not a design value.
func FindBareNumbers(value string, allowOne bool) []string {
	value = varCallPattern.ReplaceAllString(value, "")
	var found []string
	for _, number := range bareNumberPattern.FindAllString(value, -1) {
		if number == "0" || allowOne && number == "1" {
			continue
		}
		found = append(found, number)
	}
	return found
}
