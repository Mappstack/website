#!/bin/sh
set -u
check=$(cd "$(dirname "$0")" && pwd)
design=$(cd "$check/.." && pwd)
product=$(cd "$1" && pwd)
status=0
cd "$check"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
(cd "$work" && go build -o "$work/check" "$check"/checks/*.go) || exit 1
for kind in literals voice restyle buttons classes components pages styles profiles; do
	case $kind in
	(restyle|classes|components|pages|styles) xargs "$work/check" "$kind" "$product" "$design" < exclusions.txt || status=1 ;;
	(profiles) xargs "$work/check" profiles "$product" "$(cat profile.txt)" < exclusions.txt || status=1 ;;
	(*) xargs "$work/check" "$kind" "$product" < exclusions.txt || status=1 ;;
	esac
done
"$work/check" usage "$product" "$design" ${2:-} || status=1
sh theme.sh "$product" $(cat theme-colors.txt) || status=1
exit $status
