#!/bin/bash
# Example plugin: nmap scanner
# Input is passed as JSON via stdin

INPUT=$(cat)
TARGET=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('target',''))" 2>/dev/null || echo "")
PORTS=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('ports','1-1000'))" 2>/dev/null || echo "1-1000")

if [ -z "$TARGET" ]; then
    echo '{"error": "target is required"}'
    exit 1
fi

nmap -p "$PORTS" -sV "$TARGET" 2>/dev/null || echo "nmap not available - this is an example plugin"
