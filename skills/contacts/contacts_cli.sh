#!/bin/bash

# Contacts CLI Skill
# 查询和管理联系人

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "search"')
NAME=$(echo "$PARAMS" | jq -r '.name // ""')
PHONE=$(echo "$PARAMS" | jq -r '.phone // ""')
EMAIL=$(echo "$PARAMS" | jq -r '.email // ""')

case "$ACTION" in
    "search")
        # 搜索联系人
        if [ -z "$NAME" ] && [ -z "$PHONE" ] && [ -z "$EMAIL" ]; then
            echo '{"error": "At least one search parameter is required: name, phone, or email"}' >&2
            exit 1
        fi
        
        # 使用AddressBook框架搜索
        if [ -n "$NAME" ] && [ "$NAME" != "null" ]; then
            RESULTS=$(osascript -e "
tell application \"Contacts\"
    set foundPeople to every person whose name contains \"$NAME\"
    set resultString to \"\"
    repeat with p in foundPeople
        set personName to name of p
        set personPhones to \"\"
        set personEmails to \"\"
        if (count of phones of p) > 0 then
            set personPhones to value of phones of p as string
        end if
        if (count of emails of p) > 0 then
            set personEmails to value of emails of p as string
        end if
        set resultString to resultString & \"{\\\"name\\\":\\\"\" & personName & \"\\\", \\\"phone\\\":\\\"\" & personPhones & \"\\\", \\\"email\\\":\\\"\" & personEmails & \"\\\"},\"
    end repeat
    if resultString is not \"\" then
        set resultString to text 1 thru -2 of resultString
    end if
    return resultString
end tell
" 2>/dev/null)
            echo "{\"contacts\": [$RESULTS]}"
        else
            echo "{\"result\": \"Search by phone or email not implemented yet\"}"
        fi
        ;;
    "list")
        # 列出所有联系人（限制数量）
        RESULTS=$(osascript -e "
tell application \"Contacts\"
    set allPeople to every person
    set resultString to \"\"
    set countLimit to 20
    set countCurrent to 0
    repeat with p in allPeople
        set countCurrent to countCurrent + 1
        if countCurrent > countLimit then
            exit repeat
        end if
        set personName to name of p
        set resultString to resultString & \"{\\\"name\\\":\\\"\" & personName & \"\\\"},\"
    end repeat
    if resultString is not \"\" then
        set resultString to text 1 thru -2 of resultString
    end if
    return resultString
end tell
" 2>/dev/null)
        echo "{\"contacts\": [$RESULTS]}"
        ;;
    "add")
        # 添加联系人
        if [ -z "$NAME" ] || [ "$NAME" = "null" ]; then
            echo '{"error": "Missing required parameter: name for add action"}' >&2
            exit 1
        fi
        
        osascript -e "
tell application \"Contacts\"
    set newPerson to make new person with properties {name:\"$NAME\"}
    if \"$PHONE\" is not \"\" and \"$PHONE\" is not \"null\" then
        make new phone at newPerson with properties {value:\"$PHONE\"}
    end if
    if \"$EMAIL\" is not \"\" and \"$EMAIL\" is not \"null\" then
        make new email at newPerson with properties {value:\"$EMAIL\"}
    end if
    save
end tell
" 2>/dev/null
        
        echo "{\"result\": \"Contact added: $NAME\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'search', 'list', or 'add'\"}" >&2
        exit 1
        ;;
esac
