#!/bin/bash
# iMessage Permission Diagnostic Script (Send-Only Mode)
# This script helps diagnose and fix permission issues for iMessage AppleScript access
# This version only tests sending capabilities - no Full Disk Access required

echo "=== iMessage Permission Diagnostic (Send-Only Mode) ==="
echo ""

echo "1. Checking if Messages.app is running..."
if pgrep -x "Messages" > /dev/null; then
    echo "   ✓ Messages.app is running"
else
    echo "   ✗ Messages.app is not running"
    echo "   → Starting Messages.app..."
    open -a Messages
    sleep 3
fi

echo ""
echo "2. Testing basic AppleScript access..."
result=$(osascript -e '
tell application "Messages"
    return "OK"
end tell
' 2>&1)

if [ "$result" = "OK" ]; then
    echo "   ✓ Basic AppleScript access works"
else
    echo "   ✗ Basic AppleScript access failed: $result"
    echo ""
    echo "   → Please grant Automation permission:"
    echo "     1. Open System Settings"
    echo "     2. Go to Privacy & Security > Automation"
    echo "     3. Find your terminal app (Terminal/iTerm/Termius/etc.)"
    echo "     4. Toggle ON the switch for Messages"
    echo ""
    echo "   → If your terminal is not listed:"
    echo "     1. Run this command: osascript -e 'tell application \"Messages\" to activate'"
    echo "     2. A permission dialog should appear"
    echo "     3. Click 'OK' to allow"
    exit 1
fi

echo ""
echo "3. Testing message sending..."
echo "   → This will send a test message to +8613580538348"
echo "   → Press Ctrl+C to skip, or wait 5 seconds..."
sleep 5

result=$(osascript -e '
tell application "Messages"
    set targetService to service "iMessage"
    try
        set theBuddy to buddy "+8613580538348" of targetService
        send "Permission test - please ignore" to theBuddy
        return "OK"
    on error errMsg
        return "ERROR: " & errMsg
    end try
end tell
' 2>&1)

if [ "$result" = "OK" ]; then
    echo "   ✓ Message sending works"
else
    echo "   ✗ Message sending failed: $result"
    echo ""
    echo "   → Please check:"
    echo "     1. You are signed in to iMessage"
    echo "     2. The recipient number is valid"
    echo "     3. Automation permission is granted"
    exit 1
fi

echo ""
echo "=== All permissions are correctly configured! ==="
echo "You can now run the integration tests with:"
echo "  cd /Users/ray/workspaces/ai-ecosystem/gort"
echo "  INTEGRATION_TEST=1 go test ./pkg/channel/imessage/... -v -run TestChannel_Integration_SendMessage"
echo ""
echo "Or run a specific test:"
echo "  INTEGRATION_TEST=1 IMSG_RECIPIENT='+8613580538348' go test ./pkg/channel/imessage/... -v -run TestChannel_Integration_SendMessage"
