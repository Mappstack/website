package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Color struct{ Red, Green, Blue, Alpha float64 }

type ContrastPair struct {
	Foreground string
	Layers     []string
	Floor      float64
	Modes      []string
}

var rootBlockPattern = regexp.MustCompile(`(?:@media\s*([^{]+)\{\s*)?:root\s*\{([^}]*)\}`)
var mediaFeaturePattern = regexp.MustCompile(`\(([^)]+)\)`)

var ThemeFeatures = map[string][]string{
	"light":      {},
	"dark":       {"prefers-color-scheme:dark"},
	"light-more": {"prefers-contrast:more"},
	"dark-more":  {"prefers-color-scheme:dark", "prefers-contrast:more"},
}
var declarationPattern = regexp.MustCompile(`(--[\w-]+)\s*:\s*([^;]+);`)
var variablePattern = regexp.MustCompile(`^var\((--[\w-]+)\)$`)
var rgbaPattern = regexp.MustCompile(`^rgba?\(([^)]*)\)$`)

func ReadThemes(designDirectory string) (map[string]map[string]string, error) {
	themes := map[string]map[string]string{}
	for name := range ThemeFeatures {
		themes[name] = map[string]string{}
	}
	for _, name := range []string{"base.css", "color.css"} {
		content, err := os.ReadFile(filepath.Join(designDirectory, "tokens", name))
		if err != nil {
			return nil, err
		}
		for _, block := range rootBlockPattern.FindAllStringSubmatch(string(content), -1) {
			for theme, features := range ThemeFeatures {
				if !MatchesTheme(block[1], features) {
					continue
				}
				for _, declaration := range declarationPattern.FindAllStringSubmatch(block[2], -1) {
					themes[theme][declaration[1]] = strings.TrimSpace(declaration[2])
				}
			}
		}
	}
	return themes, nil
}

func MatchesTheme(query string, features []string) bool {
	for _, feature := range mediaFeaturePattern.FindAllStringSubmatch(strings.ReplaceAll(query, " ", ""), -1) {
		if !slices.Contains(features, feature[1]) {
			return false
		}
	}
	return true
}

func ResolveColor(theme map[string]string, name string) (Color, error) {
	value, found := theme[name]
	if !found {
		return Color{}, fmt.Errorf("%s is not defined", name)
	}
	if match := variablePattern.FindStringSubmatch(value); match != nil {
		return ResolveColor(theme, match[1])
	}
	return ParseColor(value)
}

func ParseColor(value string) (Color, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "transparent" {
		return Color{}, nil
	}
	if strings.HasPrefix(value, "#") && len(value) == 7 {
		channels := make([]float64, 3)
		for index := range channels {
			channel, err := strconv.ParseUint(value[1+index*2:3+index*2], 16, 8)
			if err != nil {
				return Color{}, err
			}
			channels[index] = float64(channel)
		}
		return Color{channels[0], channels[1], channels[2], 1}, nil
	}
	if match := rgbaPattern.FindStringSubmatch(value); match != nil {
		parts := strings.Split(match[1], ",")
		channels := []float64{0, 0, 0, 1}
		for index, part := range parts {
			number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil {
				return Color{}, err
			}
			channels[index] = number
		}
		return Color{channels[0], channels[1], channels[2], channels[3]}, nil
	}
	return Color{}, fmt.Errorf("unreadable color %q", value)
}

func BlendColor(top, bottom Color) Color {
	mix := func(over, under float64) float64 { return over*top.Alpha + under*(1-top.Alpha) }
	return Color{mix(top.Red, bottom.Red), mix(top.Green, bottom.Green), mix(top.Blue, bottom.Blue), 1}
}

func MeasureLuminance(color Color) float64 {
	linear := func(channel float64) float64 {
		channel /= 255
		if channel <= 0.03928 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(color.Red) + 0.7152*linear(color.Green) + 0.0722*linear(color.Blue)
}

func MeasureContrast(first, second Color) float64 {
	lighter, darker := MeasureLuminance(first), MeasureLuminance(second)
	if darker > lighter {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

func ListContrastPairs() []ContrastPair {
	all := []string{"light", "dark", "light-more", "dark-more"}
	standard := []string{"light", "dark"}
	more := []string{"light-more", "dark-more"}
	surfaces := []string{"--color-background", "--color-background-grouped", "--color-surface", "--color-surface-raised", "--color-surface-sunken", "--color-surface-canvas"}
	plainSurfaces := []string{"--color-background", "--color-surface", "--color-surface-raised", "--color-surface-canvas"}
	controlBases := []string{"--color-surface", "--color-background-grouped", "--color-surface-raised"}
	fills := []string{"--color-fill-subtle", "--color-fill", "--color-fill-strong", "--color-selected"}
	var pairs []ContrastPair
	addPair := func(foreground string, floor float64, modes []string, layers ...string) {
		pairs = append(pairs, ContrastPair{foreground, layers, floor, modes})
	}
	for _, foreground := range []string{"--color-text", "--color-text-secondary", "--color-primary"} {
		for _, surface := range surfaces {
			addPair(foreground, 4.5, all, surface)
		}
	}
	for _, foreground := range []string{"--color-error-text", "--color-warn-text", "--color-success-text"} {
		for _, surface := range plainSurfaces {
			addPair(foreground, 4.5, all, surface)
		}
	}
	for _, foreground := range []string{"--color-text", "--color-primary"} {
		for _, base := range controlBases {
			for _, fill := range fills {
				addPair(foreground, 4.5, all, base, fill)
			}
		}
	}
	for _, layers := range [][]string{
		{"--color-surface", "--color-fill"}, {"--color-background-grouped", "--color-fill"}, {"--color-surface-raised", "--color-fill"},
		{"--color-surface", "--color-fill-subtle"}, {"--color-surface-raised", "--color-fill-subtle"}, {"--color-background", "--color-fill-subtle"},
		{"--color-surface", "--color-selected"}, {"--color-background-grouped", "--color-selected"}, {"--color-background", "--color-selected"},
	} {
		addPair("--color-text-secondary", 4.5, all, layers...)
	}
	for _, surface := range plainSurfaces {
		addPair("--color-accent", 4.5, all, surface)
	}
	for _, fill := range []string{"--color-primary", "--color-primary-hover"} {
		addPair("--color-on-primary", 4.5, all, fill)
	}
	addPair("--color-on-error", 3, standard, "--color-error")
	addPair("--color-on-error", 4.5, more, "--color-error")
	for _, base := range surfaces {
		addPair("--color-on-scrim", 4.5, all, base, "--color-scrim")
	}
	for _, fill := range []string{"--color-avatar-start", "--color-avatar-end"} {
		addPair("--color-text", 4.5, all, fill)
	}
	for _, foreground := range []string{"--color-focus", "--color-border-strong", "--color-error"} {
		for _, surface := range surfaces {
			addPair(foreground, 3, all, surface)
		}
	}
	for _, hue := range []string{"blue", "indigo", "purple", "violet", "graphite", "pink", "red", "orange", "yellow", "green", "mint", "teal", "brown", "gray", "neutral"} {
		addPair("--color-diagram-"+hue+"-text", 3, standard, "--color-diagram-"+hue)
		addPair("--color-diagram-"+hue+"-text", 4.5, more, "--color-diagram-"+hue)
	}
	for _, depth := range []string{"--color-diagram-band", "--color-diagram-depth-1", "--color-diagram-depth-2", "--color-diagram-depth-3", "--color-diagram-depth-4", "--color-diagram-depth-5"} {
		addPair("--color-text", 4.5, all, depth)
	}
	for _, depth := range []string{"--color-diagram-depth-1", "--color-diagram-depth-2", "--color-diagram-depth-3", "--color-diagram-depth-4"} {
		addPair("--color-text-secondary", 4.5, []string{"light", "light-more"}, depth)
	}
	for _, depth := range []string{"--color-diagram-depth-1", "--color-diagram-depth-2"} {
		addPair("--color-text-secondary", 4.5, []string{"dark", "dark-more"}, depth)
	}
	for _, ring := range []string{"1", "2", "3"} {
		for _, end := range []string{"start", "end"} {
			for _, surface := range plainSurfaces {
				addPair("--color-ring-"+ring+"-"+end, 3, all, surface)
			}
		}
	}
	addPair("--color-text-secondary", 4.5, all, "--color-diagram-neutral")
	addPair("--color-border-strong", 3, all, "--color-diagram-depth-1")
	return pairs
}

func CheckContrast(designDirectory string) ([]string, error) {
	themes, err := ReadThemes(designDirectory)
	if err != nil {
		return nil, err
	}
	report := len(os.Args) > 2 && os.Args[2] == "report"
	var problems []string
	for _, pair := range ListContrastPairs() {
		for _, mode := range pair.Modes {
			theme := themes[mode]
			background := Color{255, 255, 255, 1}
			if strings.HasPrefix(mode, "dark") {
				background = Color{0, 0, 0, 1}
			}
			for _, layer := range pair.Layers {
				color, err := ResolveColor(theme, layer)
				if err != nil {
					return nil, err
				}
				background = BlendColor(color, background)
			}
			foreground, err := ResolveColor(theme, pair.Foreground)
			if err != nil {
				return nil, err
			}
			ratio := MeasureContrast(BlendColor(foreground, background), background)
			line := fmt.Sprintf("%s %s on %s: %.2f (floor %.1f)", mode, pair.Foreground, strings.Join(pair.Layers, " + "), ratio, pair.Floor)
			if ratio < pair.Floor {
				problems = append(problems, "contrast: "+line)
			} else if report {
				fmt.Println(line)
			}
		}
	}
	return problems, nil
}
