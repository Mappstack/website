package main

import (
	"fmt"
	"strings"
)

func ReadBackgrounds(designDirectory string) (string, error) {
	themes, err := ReadThemes(designDirectory)
	if err != nil {
		return "", err
	}
	var backgrounds []string
	for _, mode := range []string{"light", "dark"} {
		color, err := ResolveColor(themes[mode], "--color-background")
		if err != nil {
			return "", err
		}
		backgrounds = append(backgrounds, FormatHex(color))
	}
	return strings.Join(backgrounds, " "), nil
}

func FormatHex(color Color) string {
	return fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
}
