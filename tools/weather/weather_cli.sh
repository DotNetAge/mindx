#!/bin/bash

PARAMS=$(cat)
CITY=$(echo "$PARAMS" | jq -r '.city // "北京"')
DAYS=$(echo "$PARAMS" | jq -r '.days // 1')

echo "{\"city\": \"$CITY\", \"current\": {\"temp\": \"25\", \"condition\": \"晴\"}, \"days\": $DAYS}"
