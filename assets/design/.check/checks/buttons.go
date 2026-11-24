package main

import (
	"fmt"
	"regexp"
	"strings"
)

var buttonTagPattern = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][\w-]*)([^<>]*?)(/?)>`)
var buttonClassPattern = regexp.MustCompile(`\bclass\s*=`)
var voidElements = map[string]bool{"input": true, "area": true, "base": true, "col": true, "embed": true, "param": true, "track": true, "wbr": true, "br": true, "img": true, "hr": true, "meta": true, "link": true, "source": true, "path": true, "use": true, "circle": true}
var smallForbidden = map[string]bool{"form": true, "list__row": true, "cell": true}

var templateActionPattern = regexp.MustCompile(`\{\{-?\s*(if|with|range|block|define|else|end)\b`)

type openElement struct {
	Name     string
	Classes  []string
	Dialog   bool
	PrevView int
}

type branchFrame struct {
	Start    map[int]int
	Best     map[int]int
	Define   bool
	PrevView int
}

func CopyCounts(counts map[int]int) map[int]int {
	copied := map[int]int{}
	for key, value := range counts {
		copied[key] = value
	}
	return copied
}

func MaxCounts(left, right map[int]int) map[int]int {
	merged := CopyCounts(left)
	for key, value := range right {
		if value > merged[key] {
			merged[key] = value
		}
	}
	return merged
}

func CheckButtons(root string, exclusions []string) ([]string, error) {
	var problems []string
	err := WalkFiles(root, exclusions, templateExtensions, func(relative, extension, text string) {
		problems = append(problems, FindButtonProblems(relative, text)...)
	})
	return problems, err
}

func FindButtonProblems(relative, markup string) []string {
	visible := BlankMatches(markup, markupHiddenPattern)
	var problems []string
	var stack []openElement
	primaries := map[int]int{}
	view := 0
	var frames []branchFrame
	events := buttonTagPattern.FindAllStringSubmatchIndex(visible, -1)
	actions := templateActionPattern.FindAllStringSubmatchIndex(visible, -1)
	var merged [][]int
	for len(events) > 0 || len(actions) > 0 {
		if len(actions) > 0 && (len(events) == 0 || actions[0][0] < events[0][0]) {
			merged = append(merged, append([]int{-1}, actions[0]...))
			actions = actions[1:]
		} else {
			merged = append(merged, events[0])
			events = events[1:]
		}
	}
	for _, tag := range merged {
		if tag[0] == -1 {
			switch word := visible[tag[3]:tag[4]]; word {
			case "else":
				if len(frames) > 0 {
					frame := &frames[len(frames)-1]
					frame.Best = MaxCounts(frame.Best, primaries)
					primaries = CopyCounts(frame.Start)
				}
			case "end":
				if len(frames) > 0 {
					frame := frames[len(frames)-1]
					frames = frames[:len(frames)-1]
					if frame.Define {
						view = frame.PrevView
					} else {
						primaries = MaxCounts(frame.Best, primaries)
					}
				}
			default:
				frame := branchFrame{Start: CopyCounts(primaries), Best: map[int]int{}, Define: word == "define", PrevView: view}
				if frame.Define {
					view = tag[1] + 1
				}
				frames = append(frames, frame)
			}
			continue
		}
		closing := visible[tag[2]:tag[3]] == "/"
		name := strings.ToLower(visible[tag[4]:tag[5]])
		attributes := visible[tag[6]:tag[7]]
		if closing {
			for index := len(stack) - 1; index >= 0; index-- {
				if stack[index].Name == name {
					if stack[index].Dialog {
						view = stack[index].PrevView
					}
					stack = stack[:index]
					break
				}
			}
			continue
		}
		var classes []string
		if at := buttonClassPattern.FindStringIndex(attributes); at != nil {
			classes = strings.FieldsFunc(ClassValue(attributes[at[1]:]), func(character rune) bool { return strings.ContainsRune(" \t\r\n\"'{}", character) })
		}
		line := LineOf(visible, tag[0])
		has := func(class string) bool {
			for _, candidate := range classes {
				if candidate == class {
					return true
				}
			}
			return false
		}
		if has("btn--primary") {
			primaries[view]++
			if primaries[view] > 1 {
				problems = append(problems, fmt.Sprintf("%s:%d: second .btn--primary in one view: a view has one primary action", relative, line))
			}
		}
		if has("btn--sm") {
			for _, parent := range stack {
				container := ""
				if smallForbidden[parent.Name] {
					container = parent.Name
				}
				for _, class := range parent.Classes {
					if smallForbidden[class] {
						container = "." + class
					}
				}
				if container != "" {
					problems = append(problems, fmt.Sprintf("%s:%d: .btn--sm inside %s: controls draw at 44px there", relative, line, container))
					break
				}
			}
		}
		if visible[tag[8]:tag[9]] == "/" || voidElements[name] {
			continue
		}
		dialog := name == "dialog"
		previousView := view
		if dialog {
			view = tag[0] + 1
		}
		stack = append(stack, openElement{name, classes, dialog, previousView})
	}
	return problems
}

func ClassValue(attributes string) string {
	attributes = strings.TrimLeft(attributes, " \t\r\n")
	if attributes == "" || attributes[0] != '"' && attributes[0] != '\'' {
		return strings.Fields(attributes + " ")[0]
	}
	quote := attributes[0]
	depth := 0
	for index := 1; index < len(attributes); index++ {
		switch {
		case strings.HasPrefix(attributes[index:], "{{"):
			depth++
			index++
		case strings.HasPrefix(attributes[index:], "}}"):
			depth--
			index++
		case attributes[index] == quote && depth <= 0:
			return attributes[1:index]
		}
	}
	return attributes[1:]
}
