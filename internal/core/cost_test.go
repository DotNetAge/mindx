package core

import (
	"testing"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name                      string
		costPer1MIn, costPer1MOut float64
		costPer1MInCache          float64
		input, output, cached     int64
		want                      float64
	}{
		{
			name:        "only input tokens",
			costPer1MIn: 10,
			input:       1_000_000,
			output:      0,
			want:        10.0,
		},
		{
			name:         "input and output",
			costPer1MIn:  5,
			costPer1MOut: 15,
			input:        1_000_000,
			output:       500_000,
			want:         5.0 + 7.5,
		},
		{
			name:        "with cached input (cache free when cache price 0)",
			costPer1MIn: 10,
			input:       1_000_000,
			output:      0,
			cached:      500_000,
			want:        5.0,
		},
		{
			name:             "cached input billed at cache price",
			costPer1MIn:      10,
			costPer1MInCache: 2,
			input:            1_000_000,
			output:           0,
			cached:           500_000,
			want:             5.0 + 2.0/1_000_000*500_000, // 输入5.0 + 缓存1.0
		},
		{
			name:   "zero cost model",
			input:  1_000_000,
			output: 1_000_000,
			want:   0.0,
		},
		{
			name:        "partial tokens",
			costPer1MIn: 10,
			input:       100,
			output:      0,
			want:        10.0 / 1_000_000 * 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCost(tt.costPer1MIn, tt.costPer1MOut, tt.costPer1MInCache, tt.input, tt.output, tt.cached)
			if got != tt.want {
				t.Errorf("CalculateCost = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestDefaultCosts(t *testing.T) {
	if DefaultInputCost != 3.0 {
		t.Errorf("DefaultInputCost = %f, want 3.0", DefaultInputCost)
	}
	if DefaultOutputCost != 15.0 {
		t.Errorf("DefaultOutputCost = %f, want 15.0", DefaultOutputCost)
	}
}
