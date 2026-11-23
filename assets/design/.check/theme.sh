#!/bin/sh
set -eu
root=$(cd "$1" && pwd)
light=$2
dark=$3
exclusions=${4:-exclusions.txt}
[ ! -f "$exclusions" ] || exclusions=$(cd "$(dirname "$exclusions")" && pwd)/$(basename "$exclusions")
problems=$(cd "$root" && grep -rnoE --include='*.html' --include='*.tmpl' --include='*.webmanifest' --exclude-dir='.?*' --exclude-dir='mock-*' --exclude-dir=node_modules --exclude-dir=vendor -e '<meta[^>]*name="theme-color"[^>]*>|"(theme|background)_color"[[:space:]]*:[[:space:]]*"[^"]*"' . | grep -v '{{' | while IFS= read -r line; do
	file=${line%%:*}
	file=${file#./}
	case /$file in
	(*/mock-*|*/.[!.]*) continue ;;
	esac
	skip=
	if [ -f "$exclusions" ]; then
		while IFS= read -r excluded; do
			case $file in
			("$excluded"|"$excluded"/*) skip=1 ;;
			esac
		done < "$exclusions"
	fi
	[ -z "$skip" ] || continue
	case "$line" in
	(*prefers-color-scheme*dark*) expected=$dark ;;
	(*) expected=$light ;;
	esac
	case "$line" in
	(*"\"$expected\""*) ;;
	(*) echo "${line#./}: theme color is not --color-background $expected" ;;
	esac
done)
[ -z "$problems" ] && exit 0
echo "$problems"
exit 1
