package main

import (
	"fmt"
	"regexp"
	"strings"
)

var scriptSourcePattern = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']([^"']*)["']`)

func CheckPages(root, designDirectory string, exclusions []string) ([]string, error) {
	design, err := ReadDesign(designDirectory)
	if err != nil {
		return nil, err
	}
	var problems []string
	err = WalkFiles(root, exclusions, templateExtensions, func(relative, extension, text string) {
		problems = append(problems, ReportFindings(relative, FindPageProblems(text, extension, design.Classes))...)
	})
	return problems, err
}

func FindPageProblems(text, extension string, designClasses map[string]bool) []Finding {
	markup := ParseMarkup(text, extension)
	var findings []Finding
	report := func(line int, format string, arguments ...any) {
		findings = append(findings, Finding{line, fmt.Sprintf(format, arguments...)})
	}
	var bodies, docs []*Element
	for _, element := range markup.Elements {
		switch {
		case element.Name == "body" && !element.Partial && UsesDesign(markup, designClasses):
			bodies = append(bodies, element)
		case element.Name == "main" && element.Has("doc"):
			docs = append(docs, element)
		case element.Name == "meta":
			name, _ := element.Attribute("name")
			content, _ := element.Attribute("content")
			switch strings.ToLower(name) {
			case "viewport":
				if strings.Contains(content, "user-scalable=no") || strings.Contains(content, "maximum-scale=1") && !strings.Contains(content, "maximum-scale=1.") {
					report(element.Line, "viewport blocks zoom: Principles 10: the user's settings win")
				}
			case "color-scheme":
				if !strings.Contains(content, "light") || !strings.Contains(content, "dark") || strings.Contains(content, "only") {
					report(element.Line, "color-scheme %s: Patterns 9: light and dark follow the system", content)
				}
			}
		}
	}
	scripts := scriptSourcePattern.FindAllStringSubmatchIndex(text, -1)
	behaviors, document := -1, -1
	for _, script := range scripts {
		switch source := text[script[2]:script[3]]; {
		case strings.HasSuffix(strings.SplitN(source, "?", 2)[0], "design/behaviors.js"):
			behaviors = script[0]
		case strings.HasSuffix(strings.SplitN(source, "?", 2)[0], "design/document.js"):
			document = script[0]
		}
	}
	for _, body := range bodies {
		findings = append(findings, FindBodyProblems(markup, body, behaviors)...)
	}
	for _, doc := range docs {
		findings = append(findings, FindDocumentProblems(markup, doc, behaviors, document)...)
	}
	for _, element := range markup.Elements {
		if element.Has("prose") || element.Has("doc__body") {
			findings = append(findings, FindSectionProblems(markup, element)...)
		}
	}
	return findings
}

func FindBodyProblems(markup *Markup, body *Element, behaviors int) []Finding {
	var findings []Finding
	report := func(line int, format string, arguments ...any) {
		findings = append(findings, Finding{line, fmt.Sprintf(format, arguments...)})
	}
	var first *Element
	for _, child := range body.Children {
		if child.Name != "script" {
			first = child
			break
		}
	}
	if first != nil {
		leading := markup.Text[body.ContentStart:first.Offset]
		if !strings.Contains(leading, "{{template") && !strings.Contains(leading, "{{ template") && !strings.Contains(leading, "{{block") && !strings.Contains(leading, "{{ block") && !first.Has("skip-link") {
			if len(ParseSelector(".skip-link").Find(body)) > 0 {
				report(first.Line, "skip link is not first in body: Skip link: a.skip-link first in body")
			} else if !body.Opaque {
				report(body.Line, "no skip link: Access: skip link first")
			}
		}
	}
	if behaviors < 0 && !body.Opaque && len(body.Children) > 0 && NeedsBehaviors(body) {
		report(body.Line, "behaviors.js is not loaded: Behaviors: a page with a dialog, menu, form, button, data attribute or popover loads it; a page of plain content and links needs no script")
	}
	var accounts, backlinks []*Element
	for _, element := range body.Descendants() {
		if _, popover := element.Attribute("popovertarget"); popover && element.Has("avatar") && element.Name == "button" {
			accounts = append(accounts, element)
		}
		if element.Has("backlink") && !element.Has("backlink--up") && !element.Within(ParseSelector(".menu")) {
			backlinks = append(backlinks, element)
		}
	}
	for _, account := range accounts {
		for _, backlink := range backlinks {
			if !account.Conditional && !backlink.Conditional && !account.ExclusiveWith(backlink) && !account.Within(ParseSelector("dialog")) && !backlink.Within(ParseSelector("dialog")) {
				report(account.Line, "account button on a page with a back link (line %d): Patterns 2: the account button shows only on top-level pages", backlink.Line)
			}
		}
	}
	return findings
}

func FindDocumentProblems(markup *Markup, doc *Element, behaviors, document int) []Finding {
	var findings []Finding
	report := func(format string, arguments ...any) {
		findings = append(findings, Finding{doc.Line, fmt.Sprintf(format, arguments...)})
	}
	if doc.Opaque {
		return findings
	}
	contents := len(ParseSelector("nav.contents").Find(doc)) > 0
	for _, call := range markup.Calls {
		contents = contents || strings.Contains(strings.ToLower(call), "contents")
	}
	switch {
	case doc.Has("doc--single") && contents:
		report("doc--single holds contents: Document: --single is without contents")
	case !doc.Has("doc--single") && !doc.DynamicClass && !contents:
		report("document without contents: Long-form: pages with more than 3 sections get contents, else main.doc--single")
	}
	if !strings.Contains(markup.ReadText(doc), "Last updated") {
		report("document without \"Last updated <Month D, YYYY>\": Long-form: every document shows when it was last updated")
	}
	faq := len(ParseSelector(".doc__item").Find(doc)) > 0 || len(ParseSelector("[data-item]").Find(doc)) > 0
	if !faq && len(ParseSelector("[data-role=reading-time]").Find(doc)) == 0 {
		report("document without [data-role=reading-time]: Long-form: every document shows its reading time")
	}
	if faq {
		help := ParseSelector(".doc__help").Find(doc)
		if len(help) == 0 || !strings.Contains(markup.ReadText(help[0]), "Still Have Questions?") {
			report("FAQ does not end with Still Have Questions?: Long-form: an FAQ ends with Still Have Questions?")
		}
	}
	series, pager := len(ParseSelector(".doc__series").Find(doc)) > 0, len(ParseSelector(".doc__pager").Find(doc)) > 0
	if series != pager {
		report("docs series without both chips and previous/next: Long-form: a docs series gets chips and previous/next")
	}
	if behaviors < 0 || document < 0 || document < behaviors {
		report("document.js is not loaded after behaviors.js: Document: load behaviors.js, then document.js, from the design folder")
	}
	return findings
}

func FindSectionProblems(markup *Markup, prose *Element) []Finding {
	root := prose
	for root.Parent != nil {
		root = root.Parent
	}
	for _, call := range markup.Calls {
		if strings.Contains(strings.ToLower(call), "contents") {
			return nil
		}
	}
	if prose.Opaque || !prose.IsWhole() {
		return nil
	}
	sections := 0
	for _, descendant := range prose.Descendants() {
		if descendant.Name == "h2" {
			sections++
		}
	}
	if sections > 3 && len(ParseSelector(".contents").Find(root)) == 0 {
		return []Finding{{prose.Line, fmt.Sprintf("%d sections and no contents: Long-form: pages with more than 3 sections get contents", sections)}}
	}
	return nil
}

func UsesDesign(markup *Markup, designClasses map[string]bool) bool {
	for _, element := range markup.Elements {
		if HasDesignClass(element, designClasses) {
			return true
		}
	}
	return false
}

// NeedsBehaviors says whether anything on the page is driven by behaviors.js: a control
// that is pressed or filled, a dialog, a popover, a tab, or an element that opts in with
// a data attribute. A page of content and links is complete without the script.
func NeedsBehaviors(body *Element) bool {
	for _, element := range body.Descendants() {
		switch element.Name {
		case "dialog", "form", "input", "select", "textarea", "button", "details":
			return true
		}
		if element.Has("menu") || element.Has("sidebar") || element.Has("contents") || element.Has("flow") || element.Has("rings") {
			return true
		}
		for _, attribute := range element.Attributes {
			name := strings.ToLower(attribute.Name)
			if strings.HasPrefix(name, "data-") || strings.HasPrefix(name, "popover") || name == "role" && strings.EqualFold(attribute.Value, "tablist") || name == "aria-keyshortcuts" {
				return true
			}
		}
	}
	return false
}
