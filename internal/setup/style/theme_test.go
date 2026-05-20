package style

import (
	"fmt"
	"testing"
)

func TestGradientTitleOutput(t *testing.T) {
	out := GradientTitle("")
	fmt.Printf("len=%d\n", len(out))
	fmt.Printf("repr=%q\n", out)
	fmt.Println(out)

	if len(out) == 0 {
		t.Fatal("GradientTitle returned empty string")
	}
	hasANSI := false
	for i := 0; i < len(out); i++ {
		if out[i] == '\x1b' {
			hasANSI = true
			break
		}
	}
	if !hasANSI {
		t.Fatal("GradientTitle output contains no ANSI escape codes")
	}
	fmt.Printf("✅ ANSI escape codes detected in output (len=%d)\n", len(out))
}
