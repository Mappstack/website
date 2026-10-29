#!/bin/sh
set -eu
root=$1
light=$2
dark=$3
problems=$(grep -rnoE --include='*.html' --include='*.tmpl' --include='*.webmanifest' --exclude-dir=.git --exclude-dir=node_modules '<meta[^>]*name="theme-color"[^>]*>|"(theme|background)_color"[[:space:]]*:[[:space:]]*"[^"]*"' "$root" | grep -v '{{' | while IFS= read -r line; do
	case "$line" in
	(*prefers-color-scheme*dark*) expected=$dark ;;
	(*) expected=$light ;;
	esac
	case "$line" in
	(*"\"$expected\""*) ;;
	(*) echo "$line: theme color is not --color-background $expected" ;;
	esac
done)
[ -z "$problems" ] && exit 0
echo "$problems"
exit 1
