package setup

import (
	"errors"
	"runtime"
	"testing"
	"time"
)

// 网络不可用时 brew/apt/winget/pip 会长时间不返回：这里验证限时执行真的会中断
// 并返回可识别的超时哨兵，供上层翻译成「暂时无法下载 + 手动安装办法」的提示。
func TestRunWithTimeoutInterruptsStalledCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("该用例依赖 sh 验证长命令中断，Windows 下跳过")
	}

	start := time.Now()
	err := runWithTimeout(150*time.Millisecond, "sh", "-c", "sleep 5")
	if !errors.Is(err, errCommandTimeout) {
		t.Fatalf("超时应返回 errCommandTimeout，实际：%v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("超时后命令没有被及时中断，实际耗时 %s", elapsed)
	}
}

func TestRunWithTimeoutKeepsNormalFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("该用例依赖 sh 模拟退出码，Windows 下跳过")
	}

	err := runWithTimeout(2*time.Second, "sh", "-c", "exit 7")
	if err == nil {
		t.Fatal("非零退出码应返回错误")
	}
	if errors.Is(err, errCommandTimeout) {
		t.Fatal("普通失败不应被当成超时，否则会把环境问题误报成网络问题")
	}
}

func TestRunWithTimeoutPassesSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("该用例依赖 sh，Windows 下跳过")
	}

	if err := runWithTimeout(2*time.Second, "sh", "-c", "exit 0"); err != nil {
		t.Fatalf("正常退出的命令不应报错：%v", err)
	}
}
