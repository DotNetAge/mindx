#!/bin/bash

read -r json_input

url=$(echo "$json_input" | jq -r '.url // empty')
proxy=$(echo "$json_input" | jq -r '.proxy // empty')
text_output=$(echo "$json_input" | jq -r '.text_output // false')

if [ -z "$url" ]; then
    echo '{"error": "Missing required parameter: url"}' >&2
    exit 1
fi

args=()
if [ -n "$proxy" ]; then
    args+=("-p" "$proxy")
fi
if [ "$text_output" = "true" ]; then
    args+=("-t")
fi
args+=("$url")

open_url "${args[@]}"
