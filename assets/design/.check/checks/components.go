package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

type StructureRule struct {
	Match      string
	Tag        string
	Classes    string
	Attributes string
	Holds      string
	Beside     string
	Lacks      string
	Rule       string
}

var structureRules = []StructureRule{
	{Match: ".btn", Tag: "button|a|label|summary|input", Rule: "Button: .btn is a button or a link"},
	{Match: ".btn--icon", Attributes: "aria-label|aria-labelledby", Rule: "Button: --icon takes an aria-label"},
	{Match: "dialog", Classes: "dialog", Rule: "Dialog: dialog.dialog"},
	{Match: ".dialog", Tag: "dialog", Holds: ".dialog__body or .outcome or .page or .navbar", Rule: "Dialog: dialog.dialog > .dialog__body"},
	{Match: ".dialog--alert", Holds: ".outcome, .outcome__title, .outcome__message, .outcome__actions", Rule: "Dialog: confirmations are dialog.dialog.dialog--alert > .outcome with a title, a message and actions"},
	{Match: ".outcome", Holds: ".outcome__title", Rule: "Outcome: .outcome > .outcome__glyph + __title + __message + __actions"},
	{Match: ".empty", Holds: ".empty__title, .empty__desc", Lacks: ".btn--primary", Rule: "Empty: a title and one sentence; the create action stays in the bar"},
	{Match: ".notice", Holds: ".notice__icon, .notice__body", Rule: "Notice: .notice > svg.icon.notice__icon + .notice__body"},
	{Match: ".search", Holds: "input.input, .search__icon", Rule: "Search: .search > svg.icon.search__icon + input.input"},
	{Match: ".select", Holds: "select.select__input, .select__caret", Rule: "Select: .select > select.select__input + svg.select__caret"},
	{Match: "input.checkbox", Beside: ".checkbox__mark", Rule: "Checkbox: input.checkbox + span.checkbox__mark"},
	{Match: "input.radio", Beside: ".radio__mark", Rule: "Radio: input.radio + span.radio__mark"},
	{Match: "input.switch", Attributes: "type=checkbox, switch", Rule: "Switch: input.switch[type=checkbox][switch]"},
	{Match: ".switch", Tag: "input", Rule: "Switch: input.switch[type=checkbox][switch]"},
	{Match: ".checkbox", Tag: "input", Rule: "Checkbox: input.checkbox + span.checkbox__mark"},
	{Match: ".radio", Tag: "input", Rule: "Radio: input.radio + span.radio__mark"},
	{Match: "kbd", Classes: "kbd", Rule: "Key cap: kbd.kbd"},
	{Match: ".kbd", Tag: "kbd", Rule: "Key cap: kbd.kbd"},
	{Match: ".backlink", Tag: "a|button", Holds: "use[href$=#chevron-left], .backlink__label", Rule: "Nav bar: a.backlink or button.backlink > chevron-left + .backlink__label"},
	{Match: ".footer", Tag: "footer", Classes: "footer--rule", Holds: ".footer__row", Rule: "Footer: footer.footer.footer--rule > .wrap.footer__row"},
	{Match: ".footer__row", Classes: "wrap", Rule: "Footer: footer.footer.footer--rule > .wrap.footer__row"},
	{Match: ".footer__links", Tag: "nav", Rule: "Footer: links sit in nav.footer__links or inline"},
	{Match: ".wordmark", Tag: "a", Holds: ".wordmark__mark, .wordmark__name", Rule: "Wordmark: a.wordmark > .wordmark__mark + .wordmark__name"},
	{Match: ".page-title", Tag: "h1", Rule: "Page: h1.page-title"},
	{Match: ".page-header", Holds: "h1.page-title", Rule: "Page header: .page-header > h1.page-title"},
	{Match: ".auth", Tag: "main", Holds: "h1.auth__title", Rule: "Auth: main.auth > h1.auth__title"},
	{Match: ".auth__title", Tag: "h1", Rule: "Auth: main.auth > h1.auth__title"},
	{Match: ".auth__form", Tag: "form", Rule: "Auth: form.auth__form"},
	{Match: ".meter", Tag: "progress", Rule: "Meter: progress.meter"},
	{Match: ".progress", Tag: "progress", Rule: "Progress: progress.progress"},
	{Match: ".checklist", Tag: "ul", Rule: "Checklist: ul.checklist"},
	{Match: ".reading", Tag: "p", Holds: ".reading__value, .reading__unit", Rule: "Reading: p.reading > .reading__value + .reading__unit"},
	{Match: ".rings", Tag: "svg", Attributes: "viewBox=0 0 240 240", Rule: "Rings: svg.rings with viewBox 0 0 240 240"},
	{Match: ".trend", Tag: "figure", Holds: ".trend__plot", Rule: "Trend: figure.trend > svg.trend__plot"},
	{Match: ".thumbnail__more", Tag: "button", Classes: "btn btn--icon btn--sm", Rule: "Card: button.btn.btn--icon.btn--sm.thumbnail__more"},
	{Match: ".faces", Tag: "nav", Rule: "Faces: nav.faces > a.face"},
	{Match: ".face", Tag: "a", Holds: ".avatar, .face__name", Rule: "Faces: a.face > .avatar + .face__name"},
	{Match: ".avatar--add", Holds: "use[href$=#add]", Rule: "Avatar: --add with the add glyph"},
	{Match: ".avatar[popovertarget]", Tag: "button", Rule: "Account: button.avatar[popovertarget]"},
	{Match: ".skip-link", Tag: "a", Rule: "Skip link: a.skip-link"},
	{Match: "[role=tab]", Attributes: "aria-controls", Rule: "Tabs: [role=tab][aria-controls]"},
	{Match: ".code--block", Tag: "pre", Classes: "code", Rule: "Code: pre.code.code--block"},
	{Match: ".input", Tag: "input|textarea", Rule: "Text field: input.input or textarea.input"},
	{Match: ".list", Tag: "ul|ol", Rule: "List: ul.list > li.list__row"},
	{Match: ".list__row", Tag: "li", Rule: "List: ul.list > li.list__row"},
	{Match: ".cell__chevron", Tag: "svg", Classes: "icon icon--sm", Rule: "List: svg.icon.icon--sm.cell__chevron"},
	{Match: ".icon", Tag: "svg", Rule: "Icons: svg.icon"},
	{Match: ".chip", Tag: "span|button|a|label", Rule: "Chip: a span, button, a or label"},
	{Match: ".tabbar", Tag: "nav", Rule: "Shell: nav.tabbar"},
	{Match: ".tabbar__tab", Holds: "svg.icon--lg", Rule: "Tab bar: .tabbar__tab > svg.icon.icon--lg + label"},
	{Match: ".sidebar", Tag: "aside", Rule: "Shell: aside.sidebar"},
	{Match: ".contents", Tag: "nav", Rule: "Document: nav.contents"},
	{Match: ".doc", Tag: "main", Holds: "h1.doc__title, .doc__meta", Rule: "Document: main.doc > .doc__header > h1.doc__title + .doc__meta"},
	{Match: ".doc__title", Tag: "h1", Rule: "Document: h1.doc__title"},
	{Match: ".doc__body", Tag: "article", Classes: "prose", Rule: "Document: article.doc__body.prose"},
	{Match: ".doc__pager", Tag: "nav", Classes: "pager", Rule: "Document: nav.doc__pager.pager"},
	{Match: ".doc__help", Tag: "aside", Rule: "Document: aside.doc__help"},
	{Match: ".flow__top", Holds: ".backlink, progress.progress", Rule: "Flow: .flow__top > .backlink + progress.progress"},
	{Match: ".flow__title", Tag: "h1", Rule: "Flow: h1.flow__title"},
	{Match: ".flow__choices", Tag: "form", Rule: "Flow: form.flow__choices"},
	{Match: ".hint", Tag: "p", Rule: "List: p.hint"},
	{Match: ".doc__item", Holds: "h3[id]", Rule: "Long-form: every question linkable"},
	{Match: ".menu--navbar", Attributes: "popover", Rule: "Navbar: .menu.menu--navbar[popover]"},
}

type compiledRule struct {
	StructureRule
	selector ElementSelector
}

var compiledRules = func() []compiledRule {
	var rules []compiledRule
	for _, rule := range structureRules {
		rules = append(rules, compiledRule{rule, ParseSelector(rule.Match)})
	}
	return rules
}()

var textualInputTypes = map[string]bool{"": true, "text": true, "email": true, "password": true, "search": true, "url": true, "tel": true, "number": true, "date": true}
var tooltipTags = map[string]bool{"button": true, "a": true, "input": true, "select": true, "textarea": true, "summary": true}
var invalidClasses = map[string]bool{"is-invalid": true, "invalid": true, "has-error": true, "is-error": true, "error": true, "input--error": true, "field--error": true}
var formClasses = map[string]bool{"input": true, "field": true, "select": true, "checkbox": true, "radio": true, "switch": true}
var paintVariants = []string{"btn--primary", "btn--plain", "btn--destructive", "btn--destructive-filled"}
var plainVerbPattern = regexp.MustCompile(`^(Edit|Share|Rename|Export)\b`)
var genericLabels = map[string]bool{"Submit": true, "Go": true, "Proceed": true, "Click Here": true, "Here": true}
var separatorPattern = regexp.MustCompile(`^[|/\\•*~–—-]+$|\s[|/•]\s`)
var serverErrorPattern = regexp.MustCompile(`(?i)server error|something went wrong|internal error|unexpected error|error 5\d\d`)
var afterCloseTagPattern = regexp.MustCompile(`^</[^>]*>`)

func IsWithinAny(element *Element, specs ...string) bool {
	for _, spec := range specs {
		if element.Within(ParseSelector(spec)) {
			return true
		}
	}
	return false
}

func FindComponentProblems(markup *Markup, design Design) []Finding {
	var findings []Finding
	report := func(element *Element, format string, arguments ...any) {
		findings = append(findings, Finding{element.Line, fmt.Sprintf(format, arguments...)})
	}
	for _, element := range markup.Elements {
		for _, rule := range compiledRules {
			if !rule.selector.Matches(element) {
				continue
			}
			if rule.Tag != "" && !ParseSelector(rule.Tag).Matches(&Element{Name: element.Name}) {
				report(element, "%s is a <%s>, not %s: %s", rule.Match, element.Name, strings.ReplaceAll(rule.Tag, "|", " or "), rule.Rule)
			}
			if !element.DynamicClass {
				for _, class := range strings.Fields(rule.Classes) {
					if !element.Has(class) {
						report(element, "%s lacks .%s: %s", rule.Match, class, rule.Rule)
					}
				}
			}
			for _, need := range SplitList(rule.Attributes) {
				if !HasAnyAttribute(element, need) {
					report(element, "%s lacks %s: %s", rule.Match, need, rule.Rule)
				}
			}
			if !markup.Tree || element.Opaque {
				continue
			}
			for _, need := range SplitList(rule.Holds) {
				if !HasDescendant(element, need) {
					report(element, "%s holds no %s: %s", rule.Match, need, rule.Rule)
				}
			}
			if rule.Beside != "" && element.Parent != nil && !IsWithinAny(element, ".chip", ".segmented__opt", ".btn", ".menu-item") && !element.Parent.Opaque && !HasDescendant(element.Parent, rule.Beside) {
				report(element, "%s has no %s beside it: %s", rule.Match, rule.Beside, rule.Rule)
			}
			for _, banned := range SplitList(rule.Lacks) {
				if HasDescendant(element, banned) {
					report(element, "%s holds %s: %s", rule.Match, banned, rule.Rule)
				}
			}
		}
		findings = append(findings, FindElementProblems(markup, element, design)...)
	}
	return findings
}

func SplitList(list string) []string {
	var parts []string
	for _, part := range strings.Split(list, ", ") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func HasAnyAttribute(element *Element, alternatives string) bool {
	for _, alternative := range strings.Split(alternatives, "|") {
		name, want, valued := strings.Cut(alternative, "=")
		if value, present := element.Attribute(name); present && (!valued || value == want || strings.Contains(value, markupDynamic)) {
			return true
		}
	}
	return false
}

func FindElementProblems(markup *Markup, element *Element, design Design) []Finding {
	var findings []Finding
	report := func(format string, arguments ...any) {
		findings = append(findings, Finding{element.Line, fmt.Sprintf(format, arguments...)})
	}
	kind, _ := element.Attribute("type")
	kind = strings.ToLower(kind)
	switch element.Name {
	case "input":
		switch {
		case textualInputTypes[kind] && !element.Has("input") && !element.Has("visually-hidden") && !IsHiddenElement(element) && !element.DynamicClass && markup.Tree && !element.Within(ParseSelector(".date-filter")) && !element.Within(ParseSelector(".search")):
			report("input without .input: Text field: a field is .input")
		case (kind == "checkbox" || kind == "radio") && !element.Has(kind) && !element.Has("switch") && !element.DynamicClass && markup.Tree && !IsWithinAny(element, ".chip", ".segmented__opt", ".btn", ".menu-item"):
			report("%s input without .%s: Checkbox and Radio: input.%s + span.%s__mark", kind, kind, kind, kind)
		}
		if kind == "email" {
			if value, present := element.Attribute("autocorrect"); !present || value != "on" {
				report("email field without autocorrect=\"on\": Text field: iOS applies text replacements only where autocorrect is on")
			}
		}
	case "select":
		if !element.Has("select__input") && !element.DynamicClass {
			report("select without .select__input: Select: .select > select.select__input")
		}
	case "textarea":
		if !element.Has("input") && !element.DynamicClass {
			report("textarea without .input: Text field: a field is .input")
		}
	case "use":
		href, _ := element.Attribute("href")
		if xlink, present := element.Attribute("xlink:href"); present {
			href = xlink
		}
		if path, name, found := strings.Cut(href, "#"); found && strings.HasSuffix(path, "icons.svg") && !strings.Contains(name, markupDynamic) && !design.Symbols[name] {
			report("glyph %s is not in icons.svg: Icons: the glyph set is closed", name)
		}
	case "kbd":
		if element.Parent != nil && (IsWithinAny(element, "button", "a", ".btn", ".chip", ".menu-item", "label") && !element.Within(ParseSelector(".menu__shortcuts"))) {
			report("kbd inside a control: Key cap: controls never show their shortcut")
		}
	case "footer":
		if !element.Has("footer") && !element.DynamicClass && (element.Parent == nil && markup.Tree || element.Parent != nil && (element.Parent.Name == "body" || element.Parent.Has("shell__main"))) {
			report("footer without .footer: Footer: footer.footer.footer--rule")
		}
	case "pre", "table":
		class := map[string]string{"pre": "code", "table": "table"}[element.Name]
		if markup.Tree && element.Parent != nil && !element.Has(class) && !element.DynamicClass && element.IsWhole() && !IsWithinAny(element, ".prose", ".doc__body") {
			report("%s without .%s: Components: every component lives in the design system", element.Name, class)
		}
	}
	if element.Name == "button" && element.Has("cell") && (kind == "submit" || kind == "") && FillsOwnForm(markup, element) {
		report("submit control styled as a cell: Forms: a form with fields submits with a .btn, never a list row")
	}
	if _, present := element.Attribute("title"); present && (tooltipTags[element.Name] || element.Has("btn") || element.Has("chip")) {
		report("%s with a title tooltip: Button: no tooltips, a control that needs one is redesigned", element.Name)
	}
	if (element.Name == "button" || element.Name == "a") && markup.Tree && !element.Opaque && strings.TrimSpace(markup.ReadText(element)) == "" && len(ParseSelector("svg").Find(element)) > 0 && !HasAnyAttribute(element, "aria-label|aria-labelledby") {
		report("icon-only %s without aria-label: Icons: an icon-only button always has an aria-label", element.Name)
	}
	for _, class := range element.Classes {
		if invalidClasses[class] && (element.Name == "input" || element.Name == "select" || element.Name == "textarea" || HasAnyClass(element, formClasses)) {
			report("invalid by class .%s: States: invalid is aria-invalid=\"true\"", class)
		}
	}
	if _, present := element.Attribute("autofocus"); present && markup.Tree && (element.Name == "button" || element.Name == "a" || element.Has("btn") || kind == "submit" || kind == "button") && IsWithinAny(element, "dialog") {
		report("autofocus on a button in a dialog: Dialog: opening one never focuses a button")
	}
	painted := 0
	for _, variant := range paintVariants {
		if element.Has(variant) {
			painted++
		}
	}
	if painted > 1 && len(element.ClassVariants) == 0 && !(painted == 2 && element.Has("btn--plain") && element.Has("btn--destructive")) {
		report("more than one paint variant on .btn: Button: one variant per button")
	}
	if element.Has("navbar__fold") && element.Has("btn--primary") {
		report("primary action folds into More: Bar actions: only plain actions fold into More on phones")
	}
	if markup.Tree && !element.Opaque && (element.Has("btn") || element.Name == "button") {
		for _, label := range markup.ReadTexts(element) {
			if strings.Contains(label, markupDynamic) {
				continue
			}
			if genericLabels[label] && (element.Has("btn--primary") || kind == "submit") {
				report("primary action named %q: Forms: named for what it does (Send, Create, Done)", label)
				break
			}
			if plainVerbPattern.MatchString(label) && element.Has("btn") && !element.Has("btn--plain") && !element.Has("btn--icon") && IsInBar(element) {
				report("%q is not .btn--plain: Bar actions: Edit, Share, Rename and Export are plain", label)
				break
			}
		}
	}
	if element.Has("dialog__body") && markup.Tree && !element.Opaque {
		findings = append(findings, FindDialogOrderProblems(markup, element)...)
	}
	if element.Has("navbar") && markup.Tree {
		findings = append(findings, FindNavbarProblems(element)...)
	}
	if element.Has("search") && element.Parent != nil && (element.Parent.Name == "main" && !element.Parent.Has("page") || element.Parent.Has("shell__main") || element.Parent.Name == "body") {
		report(".search is not under .page: Search: a search at the top of a page sits under .page > .search")
	}
	if element.Has("segmented") && markup.Tree && !element.Opaque && !HasMarkedOption(element) {
		report(".segmented marks no selected option: Segmented: one option is aria-selected=\"true\", aria-current=\"page\" or a checked radio")
	}
	if element.Has("dialog--alert") && markup.Tree && !element.Opaque {
		findings = append(findings, FindAlertProblems(markup, element)...)
	}
	if element.Has("outcome") && markup.Tree && !element.Opaque {
		findings = append(findings, FindOutcomeProblems(markup, element)...)
	}
	if element.Has("outcome__glyph") && markup.Tree && !element.DynamicClass && !element.Opaque {
		if !element.Has("outcome__glyph--success") && !element.Has("outcome__glyph--warn") && !element.Has("outcome__glyph--error") && len(ParseSelector(".spinner").Find(element)) == 0 {
			report(".outcome__glyph is not success, warn or error: Outcome: a glyph is --success, --warn or --error, or a spinner while in progress")
		}
	}
	if element.Has("footer") && element.Name == "footer" && markup.Tree && !element.Opaque {
		findings = append(findings, FindFooterProblems(markup, element)...)
	}
	if (element.Name == "form" || element.Has("form") || element.Has("auth__form")) && markup.Tree && !element.Opaque {
		findings = append(findings, FindFormProblems(markup, element)...)
	}
	if element.Has("avatar") && element.Name == "button" {
		findings = append(findings, FindAccountProblems(element)...)
	}
	if element.Has("menu--navbar") && markup.Tree && !element.Opaque {
		items := ParseSelector(".menu-item").Find(element)
		for index, item := range items {
			if !HasText(markup.ReadTexts(item), "Sign Out") {
				continue
			}
			if index != len(items)-1 {
				report("Sign Out is not the last item: Account: the menu ends with Sign Out")
			}
			if !HasDescendant(element, ".menu__header") || !HasDescendant(element, ".menu-divider") {
				report("account menu without .menu__header and .menu-divider: Account: .menu__header, links, .menu-divider, Sign Out")
			}
		}
	}
	return findings
}

func IsHiddenElement(element *Element) bool {
	_, hidden := element.Attribute("hidden")
	label, _ := element.Attribute("aria-hidden")
	return hidden || label == "true"
}

func HasAnyClass(element *Element, set map[string]bool) bool {
	for _, class := range element.Classes {
		if set[class] {
			return true
		}
	}
	return false
}

func ChooseAlternatives(elements []*Element) []*Element {
	var chosen []*Element
	for _, element := range elements {
		exclusive := false
		for _, kept := range chosen {
			exclusive = exclusive || element.ExclusiveWith(kept)
		}
		if !exclusive {
			chosen = append(chosen, element)
		}
	}
	return chosen
}

func FindButtons(element *Element) []*Element {
	var buttons []*Element
	for _, descendant := range element.Descendants() {
		if descendant.Name == "button" || descendant.Has("btn") {
			buttons = append(buttons, descendant)
		}
	}
	return ChooseAlternatives(buttons)
}

func FindAlertProblems(markup *Markup, alert *Element) []Finding {
	var findings []Finding
	report := func(element *Element, format string, arguments ...any) {
		findings = append(findings, Finding{element.Line, fmt.Sprintf(format, arguments...)})
	}
	buttons := FindButtons(alert)
	if len(buttons) == 0 || len(buttons) > 2 {
		report(alert, "alert holds %d buttons: Dialog: an alert has OK, or the confirm button above Cancel", len(buttons))
		return findings
	}
	glyphs := ParseSelector(".outcome__glyph").Find(alert)
	if len(buttons) == 1 {
		for _, label := range markup.ReadTexts(buttons[0]) {
			if label != "OK" && !strings.Contains(label, markupDynamic) {
				report(buttons[0], "alert button is %q: Feedback: an alert has one OK", label)
			}
		}
		for _, glyph := range glyphs {
			if !glyph.DynamicClass && !glyph.Has("outcome__glyph--error") {
				report(glyph, "alert with one OK has no error glyph: Feedback: an alert for a failed action has the error glyph")
			}
		}
		return findings
	}
	var cancel, accept *Element
	for _, button := range buttons {
		if HasText(markup.ReadTexts(button), "Cancel") {
			cancel = button
		} else {
			accept = button
		}
	}
	if cancel == nil || accept == nil {
		return findings
	}
	if cancel.Offset < accept.Offset {
		report(cancel, "Cancel above the confirm button: Dialog: the confirm button sits above Cancel")
	}
	if !accept.Has("btn--primary") && !accept.Has("btn--destructive-filled") && !accept.DynamicClass {
		report(accept, "confirm button is neither .btn--primary nor .btn--destructive-filled: Dialog: confirm is btn--destructive-filled or btn--primary")
	}
	for _, glyph := range glyphs {
		if accept.Has("btn--destructive-filled") && !glyph.DynamicClass && !glyph.Has("outcome__glyph--warn") {
			report(glyph, "destructive confirmation has no warn glyph: Dialog: warn glyph for destructive ones")
		}
	}
	return findings
}

func ReadFirstTexts(markup *Markup, outcome *Element, spec string) []string {
	if found := ParseSelector(spec).Find(outcome); len(found) > 0 {
		return markup.ReadTexts(found[0])
	}
	return nil
}

func FindOutcomeProblems(markup *Markup, outcome *Element) []Finding {
	var findings []Finding
	report := func(format string, arguments ...any) {
		findings = append(findings, Finding{outcome.Line, fmt.Sprintf(format, arguments...)})
	}
	codes := ReadFirstTexts(markup, outcome, ".outcome__code")
	titles := ReadFirstTexts(markup, outcome, ".outcome__title")
	messages := ReadFirstTexts(markup, outcome, ".outcome__message")
	notFound, serverError := false, false
	for _, text := range codes {
		notFound = notFound || strings.Contains(text, "404")
	}
	for _, text := range titles {
		notFound = notFound || strings.HasSuffix(strings.ToLower(text), "not found")
	}
	for _, text := range append(titles, messages...) {
		serverError = serverError || serverErrorPattern.MatchString(text)
	}
	if serverError && !notFound {
		reference := false
		for _, text := range codes {
			reference = reference || strings.HasPrefix(text, "Reference")
		}
		if !reference {
			report("server error without .outcome__code Reference: Feedback: a server error shows .outcome__code \"Reference\" and the 8 characters the server logged")
		}
	}
	if !notFound {
		return findings
	}
	if len(ParseSelector(".outcome__glyph").Find(outcome)) > 0 {
		report("not found outcome with a glyph: Not found: no glyph, outcome glyphs mean success, warning, error or progress")
	}
	for _, code := range codes {
		if code != "Error 404" && !strings.Contains(code, markupDynamic) {
			report("not found code is %q: Not found: .outcome__code \"Error 404\"", code)
		}
	}
	for _, title := range titles {
		if title != "Page Not Found" && !strings.Contains(title, markupDynamic) {
			report("not found title is %q: Not found: \"Page Not Found\"", title)
		}
	}
	if buttons := len(FindButtons(outcome)); buttons != 1 {
		report("not found outcome holds %d buttons: Not found: one button back to the product", buttons)
	}
	return findings
}

func FindFooterProblems(markup *Markup, footer *Element) []Finding {
	var findings []Finding
	rows := ParseSelector(".footer__row").Find(footer)
	if len(rows) == 0 {
		return findings
	}
	row := rows[0]
	text := markup.ReadText(row)
	if strings.Contains(text, markupDynamic) && !strings.Contains(text, "©") {
		return findings
	}
	if !strings.Contains(text, "©") {
		return append(findings, Finding{footer.Line, "footer has no © line: Footer: the © line comes last"})
	}
	tail, last := "", ""
	if len(row.Children) > 0 {
		child := row.Children[len(row.Children)-1]
		last = markup.ReadText(child)
		after := afterCloseTagPattern.ReplaceAllString(markup.Text[child.ContentEnd:row.ContentEnd], "")
		tail = strings.TrimSpace(markupTagPattern.ReplaceAllString(after, " "))
		if tail != "" {
			last = tail
		}
	}
	if len(row.Children) > 0 && !strings.Contains(last, "©") {
		findings = append(findings, Finding{row.Line, "© line is not last in the footer: Footer: the © line comes last"})
	}
	for _, chunk := range markupTagPattern.Split(markup.Text[footer.ContentStart:footer.ContentEnd], -1) {
		chunk = strings.TrimSpace(markupSpacePattern.ReplaceAllString(html.UnescapeString(markupValuePattern.ReplaceAllString(chunk, "")), " "))
		if chunk != "" && separatorPattern.MatchString(chunk) && chunk != "·" {
			findings = append(findings, Finding{footer.Line, fmt.Sprintf("footer separator %q: Footer: separators are spaced middle dots", chunk)})
		}
	}
	return findings
}

func FindFormProblems(markup *Markup, form *Element) []Finding {
	loose := Finding{form.Line, "form opens with a loose line of text: Forms: explanations sit in the footer of the group they explain"}
	end := form.ContentEnd
	for _, child := range form.Children {
		if kind, _ := child.Attribute("type"); child.Name == "script" || child.Name == "input" && kind == "hidden" {
			continue
		}
		if child.Name == "p" && (len(child.Classes) == 0 || child.Has("hint")) {
			loose.Line = child.Line
			return []Finding{loose}
		}
		end = child.Offset
		break
	}
	leading := markupTagPattern.ReplaceAllString(markup.Text[form.ContentStart:end], " ")
	if strings.TrimSpace(markupValuePattern.ReplaceAllString(leading, "")) != "" {
		return []Finding{loose}
	}
	return nil
}

func FindAccountProblems(account *Element) []Finding {
	if _, present := account.Attribute("popovertarget"); !present || account.Parent == nil || !account.IsWhole() {
		return nil
	}
	var bar *Element
	for parent := account.Parent; parent != nil; parent = parent.Parent {
		if parent.Has("navbar__right") {
			bar = parent
			break
		}
	}
	if bar == nil {
		return []Finding{{account.Line, "account button outside .navbar__right: Account: last in .navbar__right"}}
	}
	current := account
	for current != bar {
		siblings := current.Parent.Children
		for _, later := range siblings[FindElementIndex(siblings, current)+1:] {
			if _, floating := later.Attribute("popover"); !floating && !later.Conditional && later.Name != "script" {
				return []Finding{{account.Line, "account button is not last in .navbar__right: Account: last in .navbar__right"}}
			}
		}
		current = current.Parent
	}
	return nil
}

func FindElementIndex(elements []*Element, target *Element) int {
	for index, element := range elements {
		if element == target {
			return index
		}
	}
	return -1
}

func CheckComponents(root, designDirectory string, exclusions []string) ([]string, error) {
	design, err := ReadDesign(designDirectory)
	if err != nil {
		return nil, err
	}
	var problems []string
	err = WalkFiles(root, exclusions, markupExtensions, func(relative, extension, text string) {
		problems = append(problems, ReportFindings(relative, FindComponentProblems(ParseMarkup(text, extension), design))...)
	})
	return problems, err
}

func FindFillableFields(scope []*Element) []*Element {
	var fields []*Element
	for _, element := range scope {
		kind, _ := element.Attribute("type")
		switch kind = strings.ToLower(kind); {
		case element.Name == "select" || element.Name == "textarea":
			fields = append(fields, element)
		case element.Name == "input" && kind != "hidden" && kind != "submit" && kind != "button" && kind != "reset" && kind != "image":
			fields = append(fields, element)
		}
	}
	return fields
}

func FillsOwnForm(markup *Markup, button *Element) bool {
	if _, other := button.Attribute("formaction"); other || button.Within(ParseSelector(".flow__options")) {
		return false
	}
	if !markup.Tree {
		start := strings.LastIndex(markup.Text[:button.Offset], "<form")
		end := strings.Index(markup.Text[button.Offset:], "</form>")
		if start < 0 || end < 0 {
			return false
		}
		var scope []*Element
		for _, element := range markup.Elements {
			if element.Offset > start && element.Offset < button.Offset+end {
				scope = append(scope, element)
			}
		}
		return len(FindFillableFields(scope)) > 0
	}
	var form *Element
	if id, tied := button.Attribute("form"); tied {
		for _, element := range markup.Elements {
			if value, present := element.Attribute("id"); element.Name == "form" && present && value == id {
				form = element
			}
		}
		if form == nil {
			return false
		}
		var scope []*Element
		for _, element := range markup.Elements {
			if value, present := element.Attribute("form"); present && value == id {
				scope = append(scope, element)
			}
		}
		return len(FindFillableFields(append(scope, form.Descendants()...))) > 0
	}
	for parent := button.Parent; parent != nil && form == nil; parent = parent.Parent {
		if parent.Name == "form" {
			form = parent
		}
	}
	return form != nil && !form.Has("flow__options") && len(FindFillableFields(form.Descendants())) > 0
}

func FindDialogOrderProblems(markup *Markup, body *Element) []Finding {
	first := func(spec string) int {
		if found := ParseSelector(spec).Find(body); len(found) > 0 {
			return found[0].Offset
		}
		return -1
	}
	title, message, actions := first(".dialog__title"), first(".dialog__message"), first(".dialog__actions")
	fields := FindFillableFields(body.Descendants())
	wrong := message >= 0 && title > message || actions >= 0 && (actions < title || actions < message)
	for _, field := range fields {
		wrong = wrong || field.Offset < title || field.Offset < message || actions >= 0 && field.Offset > actions
	}
	if wrong {
		return []Finding{{body.Line, "dialog body out of order: Dialog: __title, __message, fields, __actions"}}
	}
	return nil
}

func FindNavbarProblems(navbar *Element) []Finding {
	last := -1
	for _, child := range navbar.Children {
		for position, class := range []string{"navbar__left", "navbar__center", "navbar__right"} {
			if child.Has(class) {
				if position < last {
					return []Finding{{child.Line, class + " is out of order: Nav bar: __left, __center, __right"}}
				}
				last = position
			}
		}
	}
	return nil
}

func HasMarkedOption(segmented *Element) bool {
	for _, option := range ParseSelector(".segmented__opt").Find(segmented) {
		if value, present := option.Attribute("aria-selected"); present && (value == "true" || strings.Contains(value, markupDynamic)) {
			return true
		}
		if value, present := option.Attribute("aria-current"); present && (value == "page" || strings.Contains(value, markupDynamic)) {
			return true
		}
		for _, input := range ParseSelector("input").Find(option) {
			for _, attribute := range input.Attributes {
				if attribute.Name == "checked" || strings.Contains(attribute.Name, markupDynamic) {
					return true
				}
			}
		}
	}
	return len(ParseSelector(".segmented__opt").Find(segmented)) == 0
}

func IsInBar(element *Element) bool {
	for parent := element.Parent; parent != nil; parent = parent.Parent {
		for _, class := range parent.Classes {
			if strings.HasSuffix(class, "bar__right") || strings.HasSuffix(class, "bar__actions") {
				return true
			}
		}
	}
	return false
}
