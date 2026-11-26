package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
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
	Names   bool
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
	{Name: "spelling: use US English", Pattern: regexp.MustCompile(`(?i)\b(organis\w*|colour\w*|behaviour\w*|favourite\w*|cancell(ed|ing)|analys(e|ed|ing)|optimis\w*|authoris\w*|customis\w*|centre|catalogue|programme|travell\w*)\b`), Names: true},
	{Name: "word: vague, say what it does or give the number", Pattern: regexp.MustCompile(`(?i)\b(seamless(ly)?|powerful|easy|intuitive|user-friendly)\b|\bcoming soon\b`)},
	{Name: "word: hype, say what it does or give the number", Pattern: regexp.MustCompile(`(?i)\b(delightful|sleek|pixel-perfect|best-in-class|world-class)\b`)},
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
	titled := FindTitledRanges(visible)
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
			if rule.Policy && policy {
				continue
			}
			for _, span := range rule.Pattern.FindAllStringIndex(text, -1) {
				if rule.Except != nil && rule.Except.MatchString(text[span[0]:span[1]]) || rule.Names && !IsDisplayText(titled, chunk.Offset) && IsProperName(text, span[0]) {
					continue
				}
				findings = append(findings, Finding{LineOf(visible, offset), fmt.Sprintf("%s in %q", rule.Name, TruncateVoice(text))})
				break
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

func FindTitledRanges(markup string) [][]int {
	var ranges [][]int
	for _, element := range ParseMarkup(markup, ".html").Elements {
		if IsTitled(element) || len(element.Name) == 2 && element.Name[0] == 'h' && element.Name[1] >= '1' && element.Name[1] <= '6' {
			ranges = append(ranges, []int{element.Offset, element.ContentEnd})
		}
	}
	return ranges
}

var nameConnectors = map[string]bool{"for": true, "of": true, "and": true, "the": true, "de": true, "&": true}

func IsCapitalWord(word string) bool {
	for _, letter := range word {
		if unicode.IsLetter(letter) {
			return unicode.IsUpper(letter)
		}
	}
	return false
}

func EndsSentence(word string) bool {
	return strings.ContainsAny(word[len(word)-1:], ".,;:!?")
}

func IsProperName(text string, start int) bool {
	if start > 0 && text[start-1] != ' ' {
		return false
	}
	words := strings.Fields(text)
	at := len(strings.Fields(text[:start]))
	if !IsCapitalWord(words[at]) {
		return false
	}
	first, last := at, at
	for first > 0 && !EndsSentence(words[first-1]) && (IsCapitalWord(words[first-1]) || nameConnectors[words[first-1]] && first > 1 && IsCapitalWord(words[first-2])) {
		first--
	}
	for last < len(words)-1 && !EndsSentence(words[last]) && (IsCapitalWord(words[last+1]) || nameConnectors[words[last+1]] && last+2 < len(words) && IsCapitalWord(words[last+2])) {
		last++
	}
	count := 0
	for index := first; index <= last; index++ {
		starts := index == 0 || EndsSentence(words[index-1])
		if IsCapitalWord(words[index]) && !(index == first && starts) {
			count++
		}
	}
	return count >= 2
}
