#!/usr/bin/env bash
# Prepares an isolated fake home for the demo GIF, so recording never reads
# (or leaks) the real machine's history. Source it from the demo/ directory:
#
#   source ./setup-demo-env.sh
#
# Used by demo.tape; works from Git Bash on Windows and plain bash on Linux.

DEMO_DIR="$(pwd)"
DEMO_HOME="$DEMO_DIR/demo-home"

rm -rf "$DEMO_HOME"
mkdir -p "$DEMO_HOME/AppData/Roaming/Microsoft/Windows/PowerShell/PSReadLine"
cp fixtures/bash_history "$DEMO_HOME/.bash_history"
cp fixtures/pwsh_history.txt "$DEMO_HOME/AppData/Roaming/Microsoft/Windows/PowerShell/PSReadLine/ConsoleHost_history.txt"

mkdir -p bin
ext=""
if [ "$OS" = "Windows_NT" ]; then
	ext=".exe"
fi
# Build only when missing so the binary can also be cross-compiled in advance
# (e.g. a linux build dropped in bin/ before recording inside WSL).
if [ ! -x "bin/ricorda$ext" ]; then
	(cd .. && go build -o "demo/bin/ricorda$ext" .)
fi

export PATH="$DEMO_DIR/bin:$PATH"
export HOME="$DEMO_HOME"
export APPDATA="$DEMO_HOME/AppData/Roaming"
if command -v cygpath >/dev/null 2>&1; then
	export USERPROFILE="$(cygpath -w "$DEMO_HOME")"
	export APPDATA="$(cygpath -w "$DEMO_HOME/AppData/Roaming")"
fi
export PS1='\[\e[35m\]❯\[\e[0m\] '
