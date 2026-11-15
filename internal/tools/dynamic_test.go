package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/goharness/tools"
)

func TestDynamicTool_InfoPassthrough(t *testing.T) {
	info := &tools.ToolInfo{Name: "get_weather", Description: "查询天气"}
	dt := NewDynamic(info, nil)
	if dt.Info() != info {
		t.Fatal("Info() did not return the injected ToolInfo")
	}
}

func TestDynamicTool_ExecuteForwards(t *testing.T) {
	wantParams := map[string]any{"city": "hangzhou"}
	wantResult := map[string]any{"temp": 25}
	wantErr := errors.New("boom")

	var gotParams map[string]any
	dt := NewDynamic(&tools.ToolInfo{Name: "t"}, func(ctx context.Context, params map[string]any) (any, error) {
		gotParams = params
		return wantResult, wantErr
	})

	result, err := dt.Execute(context.Background(), wantParams)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute err = %v, want %v", err, wantErr)
	}
	gotResult, ok := result.(map[string]any)
	if !ok || len(gotResult) != 1 || gotResult["temp"] != 25 {
		t.Fatalf("Execute result = %v, want %v", result, wantResult)
	}
	if gotParams["city"] != "hangzhou" {
		t.Fatalf("params not forwarded: %v", gotParams)
	}
}

func TestDynamicTool_NilFnReturnsSentinel(t *testing.T) {
	dt := NewDynamic(&tools.ToolInfo{Name: "placeholder"}, nil)
	result, err := dt.Execute(context.Background(), nil)
	if result != nil {
		t.Fatalf("result = %v, want nil", result)
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("err = %v, want ErrNotImplemented", err)
	}
	if ErrNotImplemented.Error() == "" {
		t.Fatal("sentinel error message empty")
	}
}

// 编译期确认 DynamicTool 实现 tools.FuncTool 接口（与源码 var _ 断言互为印证）。
var _ tools.FuncTool = NewDynamic(&tools.ToolInfo{Name: "x"}, nil)
