package main

import (
	"fmt"
	"strings"
)

type ProfileRule struct {
	Profiles string
	Match    string
	Rule     string
	Inside   string
}

var profileRules = []ProfileRule{
	{"app", ".empty__icon", "Profiles: app uses empty states without glyphs", ""},
	{"app", ".sidebar-item__icon", "Profiles: app uses a text-only sidebar", ""},
	{"app ayuvu", ".wordmark--lg", "Profiles: app and ayuvu use the small wordmark", ""},
	{"website ayuvu", ".sidebar", "Profiles: website and ayuvu use the nav bar only", ""},
	{"website ayuvu", ".tabbar", "Profiles: website and ayuvu use the nav bar only", ""},
	{"app ayuvu", ".input--inset", "Profiles: app and ayuvu use stacked forms, stackure grouped forms", ""},
	{"app ayuvu", ".select--inset", "Profiles: app and ayuvu use stacked forms, stackure grouped forms", ""},
	{"stackure", ".wordmark--lg", "Profiles: stackure uses the small wordmark inside", "body.shell"},
}

func CheckProfiles(root, profile string, exclusions []string) ([]string, error) {
	var problems []string
	err := WalkFiles(root, exclusions, markupExtensions, func(relative, extension, text string) {
		problems = append(problems, ReportFindings(relative, FindProfileProblems(ParseMarkup(text, extension), profile))...)
	})
	return problems, err
}

func FindProfileProblems(markup *Markup, profile string) []Finding {
	var findings []Finding
	for _, element := range markup.Elements {
		for _, rule := range profileRules {
			if !HasWord(rule.Profiles, profile) || !ParseSelector(rule.Match).Matches(element) || rule.Inside != "" && !element.Within(ParseSelector(rule.Inside)) {
				continue
			}
			findings = append(findings, Finding{element.Line, fmt.Sprintf("%s in the %s profile: %s", rule.Match, profile, rule.Rule)})
		}
		if element.Has("wordmark") && !element.Has("wordmark--lg") && !element.DynamicClass && (profile == "website" || profile == "stackure" && element.Within(ParseSelector("body.site"))) {
			findings = append(findings, Finding{element.Line, fmt.Sprintf(".wordmark without --lg in the %s profile: Profiles: the large wordmark on public pages", profile)})
		}
		if profile == "stackure" && markup.Tree && !element.Opaque {
			switch {
			case element.Has("empty") && len(ParseSelector(".empty__icon").Find(element)) == 0:
				findings = append(findings, Finding{element.Line, ".empty without a glyph in the stackure profile: Profiles: stackure uses empty states with glyphs"})
			case element.Has("sidebar-item") && !element.Has("sidebar-item--root") && !element.Has("sidebar-item--child") && len(ParseSelector(".sidebar-item__icon").Find(element)) == 0:
				findings = append(findings, Finding{element.Line, ".sidebar-item without a glyph in the stackure profile: Profiles: stackure uses a sidebar with glyphs"})
			}
		}
		if profile == "ayuvu" && markup.Tree && !element.Opaque && element.Has("empty") && !element.Has("doc__noresults") && len(ParseSelector(".empty__icon").Find(element)) == 0 {
			findings = append(findings, Finding{element.Line, ".empty without a glyph in the ayuvu profile: Profiles: ayuvu uses empty states with glyphs"})
		}
	}
	return findings
}

func HasWord(list, word string) bool {
	for _, candidate := range strings.Fields(list) {
		if candidate == word {
			return true
		}
	}
	return false
}
