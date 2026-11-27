package main

import (
	"regexp"
	"strings"
)

const validationRule = "Gate and Validation: the design library's submit gate and inline errors skip any form with noValidate, so its primary button stays disabled and its errors never show"

var validationExtensions = map[string]bool{".html": true, ".tmpl": true, ".js": true, ".mjs": true, ".jsx": true, ".ts": true, ".tsx": true, ".go": true, ".py": true}
var validationScriptExtensions = map[string]bool{".js": true, ".mjs": true, ".jsx": true, ".ts": true, ".tsx": true}

var noValidatePropertyPattern = regexp.MustCompile("(?:\\.\\s*noValidate\\b|\\[\\s*[\"'`]noValidate[\"'`]\\s*\\])\\s*=")
var noValidateKeyPattern = regexp.MustCompile(`\bnoValidate\s*:\s*(true|!0|1)\b`)
var noValidateCallPattern = regexp.MustCompile("(?i)\\.\\s*(setAttribute|toggleAttribute|attr|prop)\\s*\\(\\s*[\"'`]novalidate[\"'`]\\s*(?:,\\s*([^)]*?))?\\s*\\)")
var formTagPattern = regexp.MustCompile(`(?i)<form\b`)
var templateCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->|\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
var scriptBlockPattern = regexp.MustCompile(`(?is)(<script\b[^>]*>)(.*?)</script>`)
var handlerAttributePattern = regexp.MustCompile(`(?is)\son[a-z]+\s*=\s*("[^"]*"|'[^']*')`)
var falseValues = map[string]bool{"false": true, "0": true, "!1": true, "null": true, "undefined": true}

func CheckValidation(root string, exclusions []string) ([]string, error) {
	var problems []string
	err := WalkFiles(root, exclusions, validationExtensions, func(relative, extension, text string) {
		problems = append(problems, ReportFindings(relative, FindValidationProblems(extension, text))...)
	})
	return problems, err
}

func FindValidationProblems(extension, text string) []Finding {
	var regions [][]int
	switch {
	case templateExtensions[extension]:
		text = BlankMatches(text, templateCommentPattern)
		for _, block := range scriptBlockPattern.FindAllStringSubmatchIndex(text, -1) {
			text = text[:block[4]] + BlankCodeComments(text[block[4]:block[5]], false) + text[block[5]:]
			regions = append(regions, []int{block[4], block[5]})
		}
		for _, handler := range handlerAttributePattern.FindAllStringSubmatchIndex(text, -1) {
			regions = append(regions, []int{handler[2], handler[3]})
		}
	case extension == ".py":
		text = BlankCodeComments(text, true)
	default:
		text = BlankCodeComments(text, false)
		if validationScriptExtensions[extension] {
			regions = [][]int{{0, len(text)}}
		}
	}
	inScript := func(offset int) bool {
		for _, region := range regions {
			if offset >= region[0] && offset < region[1] {
				return true
			}
		}
		return false
	}
	var findings []Finding
	script := func(offset int) {
		findings = append(findings, Finding{LineOf(text, offset), "sets noValidate on a form: " + validationRule})
	}
	for _, match := range noValidatePropertyPattern.FindAllStringIndex(text, -1) {
		rest := text[match[1]:]
		if strings.HasPrefix(rest, "=") || !inScript(match[0]) {
			continue
		}
		if end := strings.IndexAny(rest, ";,)}\n"); end >= 0 {
			rest = rest[:end]
		}
		if !falseValues[strings.TrimSpace(rest)] {
			script(match[0])
		}
	}
	if validationScriptExtensions[extension] {
		for _, match := range noValidateKeyPattern.FindAllStringIndex(text, -1) {
			script(match[0])
		}
	}
	for _, match := range noValidateCallPattern.FindAllStringSubmatchIndex(text, -1) {
		if !inScript(match[0]) {
			continue
		}
		method := strings.ToLower(text[match[2]:match[3]])
		given := match[4] >= 0
		value := ""
		if given {
			value = strings.TrimSpace(text[match[4]:match[5]])
		}
		switch {
		case method == "setattribute",
			method == "toggleattribute" && !falseValues[value],
			given && !falseValues[value]:
			script(match[0])
		}
	}
	for _, match := range formTagPattern.FindAllStringIndex(text, -1) {
		nameEnd, end, ok := ScanTag(text, match[0])
		if !ok || !strings.EqualFold(text[match[0]+1:nameEnd], "form") {
			continue
		}
		for _, attribute := range ParseAttributes(text[nameEnd:end-1], extension) {
			if strings.EqualFold(attribute.Name, "novalidate") && attribute.Value != "{false}" {
				findings = append(findings, Finding{LineOf(text, match[0]), "<form " + attribute.Name + ">: " + validationRule})
			}
		}
	}
	return findings
}

func BlankCodeComments(text string, hash bool) string {
	blanked := []byte(text)
	blank := func(start, end int) {
		for index := start; index < end; index++ {
			if blanked[index] != '\n' {
				blanked[index] = ' '
			}
		}
	}
	var quote byte
	for index := 0; index < len(text); index++ {
		character := text[index]
		switch {
		case quote != 0:
			switch {
			case character == '\\':
				index++
			case character == quote, character == '\n' && quote != '`':
				quote = 0
			}
		case character == '"' || character == '\'' || character == '`':
			quote = character
		case hash && character == '#', !hash && strings.HasPrefix(text[index:], "//"):
			end := strings.IndexByte(text[index:], '\n')
			if end < 0 {
				end = len(text) - index
			}
			blank(index, index+end)
			index += end - 1
		case !hash && strings.HasPrefix(text[index:], "/*"):
			end := len(text)
			if close := strings.Index(text[index+2:], "*/"); close >= 0 {
				end = index + 2 + close + 2
			}
			blank(index, end)
			index = end - 1
		}
	}
	return string(blanked)
}
