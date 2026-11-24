package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var hexColorPattern = regexp.MustCompile(`(^|[\s:,("'=])#([0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b`)
var colorFunctionPattern = regexp.MustCompile(`\b(rgba?|hsla?)\(`)
var styleDeclarationPattern = regexp.MustCompile(`([a-z-]+)\s*:\s*([^;{}"]+)`)
var styleAttributePattern = regexp.MustCompile(`style="([^"]*)"`)
var lengthPattern = regexp.MustCompile(`(^|[^\w.-])(-?\d*\.?\d+)(px|rem|em|ms|s)\b`)
var numberPattern = regexp.MustCompile(`^-?\d*\.?\d+$`)
var mediaConditionPattern = regexp.MustCompile(`@(media|supports|container)[^{]*`)
var timerCallPattern = regexp.MustCompile(`\bset(Timeout|Interval)\s*\(`)
var numericDelayPattern = regexp.MustCompile(`^[\d\s.+\-*/()]+$`)
var hoverMediaPattern = regexp.MustCompile(`^@media[^{]*\(\s*hover\s*:\s*hover\s*\)`)
var styleBlockPattern = regexp.MustCompile(`(?s)<style[^>]*>(.*?)</style>`)
var commentPattern = regexp.MustCompile(`(?s)/\*.*?\*/`)

var closedProperties = map[string]bool{
	"font-size": true, "font-weight": true, "line-height": true, "letter-spacing": true,
	"border-radius": true, "padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true, "padding-block": true, "padding-inline": true,
	"margin": true, "margin-top": true, "margin-right": true, "margin-bottom": true, "margin-left": true, "margin-block": true, "margin-inline": true,
	"gap": true, "row-gap": true, "column-gap": true, "height": true, "min-height": true, "max-height": true, "width": true, "min-width": true, "max-width": true,
	"top": true, "right": true, "bottom": true, "left": true, "inset": true,
	"transition": true, "transition-duration": true, "transition-delay": true, "animation": true, "animation-delay": true, "animation-duration": true, "z-index": true, "opacity": true, "box-shadow": true,
	"outline": true, "outline-offset": true, "stroke-width": true, "translate": true,
}

var allowedLengths = map[string]bool{"1px": true, "-1px": true}

var scannedExtensions = map[string]bool{".css": true, ".js": true, ".html": true, ".go": true, ".tmpl": true, ".svg": true, ".json": true}

func CheckLiterals(root string, exclusions []string) ([]string, error) {
	var problems []string
	err := WalkFiles(root, exclusions, scannedExtensions, func(relative, extension, text string) {
		for index, line := range strings.Split(text, "\n") {
			if strings.Contains(line, `name="theme-color"`) {
				continue
			}
			for _, finding := range FindLiterals(extension, line) {
				problems = append(problems, fmt.Sprintf("%s:%d: %s", relative, index+1, finding))
			}
		}
		for _, finding := range FindTimelineProblems(ReadStyleRules(text, extension)) {
			problems = append(problems, fmt.Sprintf("%s:%d: %s", relative, finding.Line, finding.Text))
		}
		for _, line := range FindBareHovers(extension, text) {
			problems = append(problems, fmt.Sprintf("%s:%d: :hover outside @media (hover:hover), so a tap leaves it on", relative, line))
		}
		if extension == ".js" {
			for _, delay := range FindTimerDelays(text) {
				problems = append(problems, fmt.Sprintf("%s:%d: timer delay %s: read it from a token, or name it if it is behavior", relative, delay.Line, delay.Value))
			}
		}
	})
	return problems, err
}

func IsExcluded(relative string, exclusions []string) bool {
	if strings.HasPrefix(filepath.Base(relative), "mock-") {
		return true
	}
	for _, exclusion := range exclusions {
		if matched, _ := filepath.Match(exclusion, relative); matched || relative == exclusion || strings.HasPrefix(relative, strings.TrimSuffix(exclusion, "/")+"/") {
			return true
		}
	}
	return false
}

func FindLiterals(extension, line string) []string {
	var findings []string
	for _, match := range hexColorPattern.FindAllStringSubmatchIndex(line, -1) {
		before := line[:match[0]+len(line[match[2]:match[3]])]
		if strings.HasSuffix(before, "href=\"") || strings.HasSuffix(before, "href='") || strings.HasSuffix(before, "url(") {
			continue
		}
		findings = append(findings, "color "+line[match[4]-1:match[5]])
	}
	if colorFunctionPattern.MatchString(line) {
		findings = append(findings, "color "+colorFunctionPattern.FindString(line)+"…)")
	}
	switch extension {
	case ".css":
		findings = append(findings, FindStyleLiterals(mediaConditionPattern.ReplaceAllString(line, ""))...)
	case ".html", ".tmpl", ".js":
		for _, attribute := range styleAttributePattern.FindAllStringSubmatch(line, -1) {
			findings = append(findings, FindStyleLiterals(attribute[1])...)
			if timelinePropertyPattern.MatchString(attribute[1]) {
				findings = append(findings, "timeline in a style attribute: Principles 9 and 10: an inline style cannot carry the media and supports guard")
			}
		}
	}
	return findings
}

func FindBareHovers(extension, text string) []int {
	var lines []int
	for _, rule := range ReadStyleRules(text, extension) {
		at := strings.Index(rule.Selector, ":hover")
		if at < 0 {
			continue
		}
		covered := false
		for _, context := range rule.Contexts {
			covered = covered || hoverMediaPattern.MatchString(context)
		}
		if !covered {
			lines = append(lines, rule.Line+strings.Count(rule.Selector[:at], "\n"))
		}
	}
	return lines
}

func FindStyleLiterals(text string) []string {
	var findings []string
	for _, declaration := range styleDeclarationPattern.FindAllStringSubmatch(text, -1) {
		property, value := declaration[1], strings.TrimSpace(declaration[2])
		if property == "scale" {
			for _, number := range FindBareNumbers(value, true) {
				findings = append(findings, property+" "+number)
			}
			continue
		}
		if strings.HasPrefix(property, "--") || !closedProperties[property] {
			continue
		}
		for _, length := range lengthPattern.FindAllStringSubmatch(value, -1) {
			literal := length[2] + length[3]
			if number, _ := strconv.ParseFloat(length[2], 64); number != 0 && !allowedLengths[literal] {
				findings = append(findings, property+" "+literal)
			}
		}
		if (property == "font-weight" || property == "z-index" || property == "opacity" || property == "line-height") && numberPattern.MatchString(value) && value != "0" && value != "1" {
			findings = append(findings, property+" "+value)
		}
	}
	return findings
}

type TimerDelay struct {
	Line  int
	Value string
}

func FindTimerDelays(script string) []TimerDelay {
	var delays []TimerDelay
	for _, call := range timerCallPattern.FindAllStringIndex(script, -1) {
		start, end := FindLastArgument(script, call[1])
		if start < 0 {
			continue
		}
		value := strings.TrimSpace(script[start:end])
		if numericDelayPattern.MatchString(value) && strings.Trim(value, "0. ") != "" {
			offset := start + len(script[start:end]) - len(strings.TrimLeft(script[start:end], " \t\n"))
			delays = append(delays, TimerDelay{strings.Count(script[:offset], "\n") + 1, value})
		}
	}
	return delays
}

func FindLastArgument(script string, open int) (int, int) {
	depth, comma := 0, -1
	var quote byte
	for index := open; index < len(script); index++ {
		character := script[index]
		switch {
		case quote != 0:
			if character == '\\' {
				index++
			} else if character == quote {
				quote = 0
			}
		case character == '"' || character == '\'' || character == '`':
			quote = character
		case character == '(' || character == '[' || character == '{':
			depth++
		case character == ')' && depth == 0:
			if comma < 0 {
				return -1, -1
			}
			return comma + 1, index
		case character == ')' || character == ']' || character == '}':
			depth--
		case character == ',' && depth == 0:
			comma = index
		}
	}
	return -1, -1
}
