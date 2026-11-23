package svc

import (
	"reflect"
	"testing"

	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
)

func TestNormalizeAskUserOptionsAcceptsObjectsAndStrings(t *testing.T) {
	raw := []any{
		map[string]any{"label": "yaml 骨架 + env 覆盖", "description": "推荐方案"},
		"只给关键字段加 env",
		map[string]any{"text": "彻底改成纯环境变量"},
		map[string]any{"description": "没有 label 的对象忽略"},
		"  ",
	}
	got := normalizeAskUserOptions(raw)
	want := []string{"yaml 骨架 + env 覆盖", "只给关键字段加 env", "彻底改成纯环境变量"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeAskUserOptions() = %#v, want %#v", got, want)
	}
}

func TestFindAskUserToolArgsByCallID(t *testing.T) {
	messages := []goharnesssession.Message{
		{Role: "assistant", ToolCalls: []goharnesssession.ToolCall{
			{ID: "call_other", Name: "Bash", Arguments: `{"command":"ls"}`},
		}},
		{Role: "assistant", ToolCalls: []goharnesssession.ToolCall{
			{ID: "call_ask", Name: "AskUser", Arguments: `{"question":"选哪个？","options":[{"label":"A"},{"label":"B"}],"multiSelect":true}`},
		}},
	}
	args, ok := findAskUserToolArgs(messages, "call_ask")
	if !ok {
		t.Fatal("findAskUserToolArgs() did not find AskUser call")
	}
	if args["question"] != "选哪个？" {
		t.Fatalf("question = %#v", args["question"])
	}
}

func TestApplyAskUserToolArgsFillsOptionsAndMultiSelect(t *testing.T) {
	data := goharnessevents.AskUserPendingData{
		Questions: []goharnessevents.AskUserQuestion{{Question: "选哪个？"}},
	}
	args := map[string]any{
		"options": []any{
			map[string]any{"label": "A"},
			map[string]any{"label": "B"},
		},
		"multiSelect": true,
	}
	applyAskUserToolArgs(&data, args)

	if got := data.Questions[0].Options; !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("Options = %#v", got)
	}
	if !data.Questions[0].MultiSelect {
		t.Fatal("MultiSelect = false, want true")
	}
}

func TestApplyAskUserToolArgsKeepsParsedOptions(t *testing.T) {
	data := goharnessevents.AskUserPendingData{
		Questions: []goharnessevents.AskUserQuestion{
			{Question: "选哪个？", Options: []string{"已解析"}, MultiSelect: false},
		},
	}
	args := map[string]any{
		"options":      []any{"不应覆盖"},
		"multi_select": true,
	}
	applyAskUserToolArgs(&data, args)

	if got := data.Questions[0].Options; !reflect.DeepEqual(got, []string{"已解析"}) {
		t.Fatalf("Options = %#v, want parsed options kept", got)
	}
	if !data.Questions[0].MultiSelect {
		t.Fatal("MultiSelect = false, want true from snake_case")
	}
}
