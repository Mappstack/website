package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var voiceHiddenPattern = regexp.MustCompile(`(?is)<(script|style|pre|code)\b.*?</(script|style|pre|code)>|<!--.*?-->|\{\{.*?\}\}`)
var voiceAttributePattern = regexp.MustCompile(`\b(?:aria-label|placeholder|alt|title)="([^"]*)"`)
var voiceTagPattern = regexp.MustCompile(`<[^>]*>`)
var voiceSpacePattern = regexp.MustCompile(`\s+`)

type VoiceRule struct {
	Name    string
	Pattern *regexp.Regexp
	Except  *regexp.Regexp
	Policy  bool
	Display bool
}

var voiceRules = []VoiceRule{
	{Name: "dash: use a period, comma or colon", Pattern: regexp.MustCompile(`[\x{2014}\x{2013}]|\w - \w`)},
	{Name: "exclamation mark", Pattern: regexp.MustCompile(`\w!(\s|$)`)},
	{Name: "emoji", Pattern: regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}]`)},
	{Name: "word: use sign in or sign out", Pattern: regexp.MustCompile(`(?i)\blog ?(in|out)\b|\blog(in|out)\b`)},
	{Name: "word: use choose", Pattern: regexp.MustCompile(`(?i)\bpick\b|\bselect one\b`)},
	{Name: "word: use app", Pattern: regexp.MustCompile(`(?i)(?:\w+ )?\b(?:web )?applications?\b(?: \w+)?`), Except: regexp.MustCompile(`(?i)\bapplication (form|programming|process|fee|letter|status|is|are)\b|\bapplications? (is|are) (read|reviewed|answered)\b|\b(job|visa|grant) application`)},
	{Name: "numeral: use digits, 1 hour, 90 days", Pattern: regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|seven|eight|nine|ten|twelve|fifteen|twenty|thirty|sixty|ninety) (second|minute|hour|day|week|month|year)s?\b`), Display: true},
	{Name: "contraction: use isn't, can't", Pattern: regexp.MustCompile(`(?i)\b(is|are|was|were|do|does|did|can|could|will|would|should|has|have|had) not\b|\bcannot\b`), Policy: true},
	{Name: "spelling: use US English", Pattern: regexp.MustCompile(`(?i)\b(organis\w*|colour\w*|behaviour\w*|favourite\w*|cancell(ed|ing)|analys(e|ed|ing)|optimis\w*|authoris\w*|customis\w*|centre|catalogue|programme|travell\w*)\b`)},
	{Name: "word: vague, say what it does or give the number", Pattern: regexp.MustCompile(`(?i)\b(seamless(ly)?|powerful|easy)\b|\bcoming soon\b`)},
	{Name: "separator: use a spaced middle dot", Pattern: regexp.MustCompile(`\w \| \w`)},
}

var policyPathPattern = regexp.MustCompile(`(?i)privacy|terms|security|policy|legal|cookie|dpa|processors`)

var voiceExtensions = map[string]bool{".html": true, ".tmpl": true}

func CheckVoice(root string, exclusions []string) ([]string, error) {
	var problems []string
	err := WalkFiles(root, exclusions, voiceExtensions, func(relative, extension, text string) {
		findings := FindVoiceProblems(text, policyPathPattern.MatchString(relative))
		markup := ParseMarkup(text, extension)
		findings = append(findings, FindTitleProblems(markup)...)
		findings = append(findings, FindDateProblems(markup)...)
		problems = append(problems, ReportFindings(relative, findings)...)
	})
	return problems, err
}

type VoiceText struct {
	Text   string
	Offset int
}

func FindVoiceProblems(markup string, policy bool) []Finding {
	visible := BlankMatches(markup, voiceHiddenPattern)
	var texts []VoiceText
	for _, attribute := range voiceAttributePattern.FindAllStringSubmatchIndex(visible, -1) {
		texts = append(texts, VoiceText{visible[attribute[2]:attribute[3]], attribute[2]})
	}
	start := 0
	for _, tag := range voiceTagPattern.FindAllStringIndex(visible, -1) {
		texts = append(texts, VoiceText{visible[start:tag[0]], start})
		start = tag[1]
	}
	texts = append(texts, VoiceText{visible[start:], start})
	displays := displayPattern.FindAllStringIndex(visible, -1)
	var findings []Finding
	for _, chunk := range texts {
		offset := chunk.Offset + len(chunk.Text) - len(strings.TrimLeft(chunk.Text, " \t\r\n"))
		text := strings.TrimSpace(voiceSpacePattern.ReplaceAllString(html.UnescapeString(chunk.Text), " "))
		if text == "" {
			continue
		}
		for _, rule := range voiceRules {
			if rule.Display && IsDisplayText(displays, chunk.Offset) {
				continue
			}
			if found := rule.Pattern.FindString(text); found != "" && !(rule.Policy && policy) && !(rule.Except != nil && rule.Except.MatchString(found)) {
				findings = append(findings, Finding{LineOf(visible, offset), fmt.Sprintf("%s in %q", rule.Name, TruncateVoice(text))})
			}
		}
	}
	return findings
}

func TruncateVoice(text string) string {
	runes := []rune(text)
	if len(runes) <= 80 {
		return text
	}
	return string(runes[:79]) + "…"
}

var displayPattern = regexp.MustCompile(`(?is)<(h1|h2)\b[^>]*>.*?</(h1|h2)>|<(\w+)\b[^>]*class="[^"]*\b(display|page-title)\b[^"]*"[^>]*>.*?</(\w+)>`)

func IsDisplayText(ranges [][]int, offset int) bool {
	for _, found := range ranges {
		if offset >= found[0] && offset < found[1] {
			return true
		}
	}
	return false
}
