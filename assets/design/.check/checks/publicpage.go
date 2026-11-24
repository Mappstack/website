package main

import (
	"fmt"
	"regexp"
	"strings"
)

var motionClasses = []string{"rise", "rise-group", "settle"}
var motionWholeTags = map[string]bool{"body": true, "main": true, "header": true, "footer": true, "nav": true, "section": true, "article": true, "aside": true, "form": true, "table": true}
var motionWholeClasses = []string{"band", "opener", "wrap", "manifesto", "invite", "tiles", "page", "doc"}
var ratioClasses = []string{"media--wide", "media--square", "media--portrait"}
var motionQueryPattern = regexp.MustCompile(`^@media\s+(?:only\s+)?(?:all\s+and\s+)?\(\s*prefers-reduced-motion\s*:\s*no-preference\s*\)(?:\s+and\s+\([^()]*\))*\s*$`)
var timelineSupportPattern = regexp.MustCompile(`^@supports\s+\(\s*animation-timeline\s*:\s*[\w-]+\([^()]*\)\s*\)\s*$`)
var innerParenthesesPattern = regexp.MustCompile(`\([^()]*\)`)
var timelinePropertyPattern = regexp.MustCompile(`(?i)(animation-timeline|animation-range|view-timeline|scroll-timeline)`)
var timelineNamePattern = regexp.MustCompile(`--[\w-]+`)
var varCallPattern = regexp.MustCompile(`var\((?:[^()]|\([^()]*\))*\)`)
var bareNumberPattern = regexp.MustCompile(`-?\d*\.?\d+`)

// The system's own named timeline: the bar's title, which has its own reduced motion fallback.
var systemTimelines = map[string]bool{"--page-title": true}

const maxRiseChildren = 6

func FindMotionProblems(markup *Markup) []Finding {
	var findings []Finding
	report := func(element *Element, format string, arguments ...any) {
		findings = append(findings, Finding{element.Line, fmt.Sprintf(format, arguments...)})
	}
	grand := false
	for _, element := range markup.Elements {
		grand = grand || element.Has("display--grand")
	}
	openers := 0
	for _, element := range markup.Elements {
		if grand && element.Has("display--hero") {
			report(element, "display--hero on a page with a display--grand headline: Page: nothing is larger than the headline, and 56 outsizes it on phones")
		}
		if element.Has("opener") && !element.Conditional {
			if openers++; openers > 1 {
				report(element, "second .opener on a page: Opener: a page has one first screen")
			}
		}
		if element.Has("opener") {
			media := 0
			for _, child := range element.Children {
				if child.Has("media") {
					if media++; media > 1 {
						report(child, "second .media in one .opener: Opener: one image beside the headline")
					}
				}
			}
		}
		if element.Has("media--drift") {
			ratio := false
			for _, class := range ratioClasses {
				ratio = ratio || element.Has(class)
			}
			if !ratio && !element.DynamicClass {
				report(element, "media--drift without media--wide, media--square or media--portrait: Scroll motion: a drifting image needs a fixed frame")
			}
		}
		var used string
		for _, class := range motionClasses {
			if element.Has(class) {
				used = class
			}
		}
		if used == "" {
			continue
		}
		whole := motionWholeTags[element.Name] || used != "rise-group" && (element.Name == "ul" || element.Name == "ol")
		for _, class := range motionWholeClasses {
			whole = whole || element.Has(class) && !(used == "rise-group" && class == "tiles")
		}
		if whole {
			report(element, ".%s on this <%s>: Scroll motion: it goes on a block shorter than the screen, never a whole region", used, element.Name)
		}
		for _, region := range []string{"section", "main", "article", ".band", ".tiles", ".wrap", ".opener", ".manifesto", ".invite"} {
			if len(ParseSelector(region).Find(element)) > 0 {
				report(element, ".%s wraps a region (%s): Scroll motion: it goes on a block shorter than the screen, never a whole region", used, region)
				break
			}
		}
		if element.Within(ParseSelector(".opener")) {
			report(element, ".%s inside .opener: Scroll motion: the first screen arrives by its own load cascade, a scroll timeline paints it part faded", used)
		}
		for _, outer := range motionClasses {
			if element.Within(ParseSelector("." + outer)) {
				report(element, ".%s inside .%s: Scroll motion: one motion per block", used, outer)
			}
		}
		if used == "settle" && !element.Has("media") {
			report(element, ".settle on a block that is not .media: Scroll motion: only a .media settles")
		}
		if used == "rise-group" && len(element.Children) > maxRiseChildren {
			report(element, ".rise-group holds %d children: Scroll motion: the cascade stops at %d", len(element.Children), maxRiseChildren)
		}
	}
	return findings
}

func FindMediaProblems(element *Element) []Finding {
	var findings []Finding
	report := func(format string, arguments ...any) {
		findings = append(findings, Finding{element.Line, fmt.Sprintf(format, arguments...)})
	}
	if element.Has("display--grand") && !(element.Name == "h1" && element.Within(ParseSelector(".opener"))) && !element.DynamicClass {
		report("display--grand outside the opener's h1: Page: only the first screen's headline is grand, nothing is ever larger than it")
	}
	for _, title := range []string{"manifesto__text", "invite__title", "tile__title"} {
		if element.Has(title) && (element.Has("display--grand") || element.Has("display--hero")) {
			report(".%s with display--grand or display--hero: Page: it is a plain .display, smaller than the headline", title)
		}
	}
	if element.Has("media--bleed") && (!element.Within(ParseSelector(".wrap")) || element.Within(ParseSelector(".tile")) || element.Has("media--round")) {
		report("media--bleed outside a .wrap, in a .tile or round: Media: it runs to the screen's edges inside the page's gutter")
	}
	if element.Name == "video" && element.Has("media__image") {
		for _, name := range []string{"autoplay", "loop"} {
			if _, present := element.Attribute(name); present {
				report("video in .media with %s: Media: a video starts by a person's press, since motion preferences cannot stop it", name)
			}
		}
	}
	if element.Name != "img" || !element.Within(ParseSelector(".media")) {
		return findings
	}
	for _, name := range []string{"alt", "width", "height"} {
		if _, present := element.Attribute(name); !present {
			report("image in .media without %s: Media: alt for the person, width and height so the page never jumps", name)
		}
	}
	loading, _ := element.Attribute("loading")
	opening := element.Within(ParseSelector(".opener"))
	switch {
	case opening && loading == "lazy":
		report("image in .opener is loading=\"lazy\": Media: the first screen's image loads at once")
	case !opening && loading != "lazy":
		report("image in .media without loading=\"lazy\": Media: only the opener's image loads at once")
	}
	return findings
}

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
					findings = append(findings, Finding{declaration.Line, declaration.Property + " without a range name: Scroll motion: name the range, or it is read as another"})
				}
			}
			if declaration.Property == "animation-range" {
				if problem := FindRangeProblem(declaration.Value); problem != "" {
					findings = append(findings, Finding{declaration.Line, "animation-range " + problem + ": Scroll motion: name the range on both ends, or the end is read as another range"})
				}
			}
			if !usesScrollTimeline(declaration) || reduced && supported {
				continue
			}
			findings = append(findings, Finding{declaration.Line, declaration.Property + " " + declaration.Value + " outside @media (prefers-reduced-motion:no-preference) and @supports (animation-timeline:view()) together: Scroll motion: it exists only where supported and motion is not reduced"})
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
