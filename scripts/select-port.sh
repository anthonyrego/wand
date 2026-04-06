#!/usr/bin/env bash
set -euo pipefail

json=$(arduino-cli board list --format json 2>/dev/null)

ports=()
labels=()
while IFS='|' read -r port name; do
    [[ -z "$port" ]] && continue
    ports+=("$port")
    labels+=("$name")
done < <(python3 -c "
import json, sys
data = json.loads(sys.stdin.read())
if isinstance(data, dict):
    entries = data.get('detected_ports', [])
else:
    entries = data
for entry in entries:
    port = entry.get('port', {}).get('address', 'unknown')
    boards = entry.get('matching_boards', [])
    name = boards[0]['name'] if boards else 'Unknown board'
    print(f'{port}|{name}')
" <<< "$json")

if [ ${#ports[@]} -eq 0 ]; then
    echo "No boards detected." >&2
    exit 1
fi

echo "Select a board:" >&2
for i in "${!ports[@]}"; do
    echo "  $((i+1))) ${labels[$i]} (${ports[$i]})" >&2
done
echo "  q) Cancel" >&2

while true; do
    read -rp "> " choice </dev/tty
    if [[ "$choice" == "q" || "$choice" == "Q" ]]; then
        echo "Cancelled." >&2
        exit 1
    fi
    if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le ${#ports[@]} ]; then
        echo "${ports[$((choice-1))]}"
        exit 0
    fi
    echo "Invalid selection." >&2
done
