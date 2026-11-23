package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: check contrast | literals <dir> [exclude...] | voice <dir> [exclude...] | restyle <product dir> <design dir> [exclude...] | usage <product dir> <design dir> [previous components.css] | buttons <dir> [exclude...] | classes|components|pages|styles <product dir> <design dir> [exclude...] | profiles <product dir> <profile> [exclude...] | background | rename <renames.json> <dir> [skip dir...]")
		os.Exit(2)
	}
	var problems []string
	var err error
	switch os.Args[1] {
	case "contrast":
		problems, err = CheckContrast(".")
	case "literals":
		problems, err = CheckLiterals(os.Args[2], os.Args[3:])
	case "restyle":
		problems, err = CheckRestyle(os.Args[2], os.Args[3], os.Args[4:])
	case "classes":
		problems, err = CheckClasses(os.Args[2], os.Args[3], os.Args[4:])
	case "components":
		problems, err = CheckComponents(os.Args[2], os.Args[3], os.Args[4:])
	case "styles":
		problems, err = CheckStyles(os.Args[2], os.Args[3], os.Args[4:])
	case "pages":
		problems, err = CheckPages(os.Args[2], os.Args[3], os.Args[4:])
	case "profiles":
		problems, err = CheckProfiles(os.Args[2], os.Args[3], os.Args[4:])
	case "buttons":
		problems, err = CheckButtons(os.Args[2], os.Args[3:])
	case "voice":
		problems, err = CheckVoice(os.Args[2], os.Args[3:])
	case "usage":
		previous := ""
		if len(os.Args) > 4 {
			previous = os.Args[4]
		}
		problems, err = CheckUsage(os.Args[2], os.Args[3], previous)
	case "background":
		var backgrounds string
		if backgrounds, err = ReadBackgrounds("."); err == nil {
			fmt.Println(backgrounds)
			return
		}
	case "rename":
		err = ApplyRenames(os.Args[2], os.Args[3], os.Args[4:])
	default:
		err = fmt.Errorf("unknown check %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, problem := range problems {
		fmt.Println(problem)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "%d problems\n", len(problems))
		os.Exit(1)
	}
}
