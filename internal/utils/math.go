package utils

import "math"

// CalculateEuclideanDistance 计算欧几里得距离
func CalculateEuclideanDistance(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var sum float64
	for i := 0; i < len(vec1); i++ {
		diff := vec1[i] - vec2[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// CalculateManhattanDistance 计算曼哈顿距离
func CalculateManhattanDistance(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var sum float64
	for i := 0; i < len(vec1); i++ {
		sum += math.Abs(vec1[i] - vec2[i])
	}

	return sum
}

// Average 计算平均值
func Average(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}
	var sum float64
	for _, n := range numbers {
		sum += n
	}
	return sum / float64(len(numbers))
}

// Max 获取最大值
func Max(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}
	max := numbers[0]
	for _, n := range numbers {
		if n > max {
			max = n
		}
	}
	return max
}

// Min 获取最小值
func Min(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}
	min := numbers[0]
	for _, n := range numbers {
		if n < min {
			min = n
		}
	}
	return min
}

// Sum 计算总和
func Sum(numbers []float64) float64 {
	var sum float64
	for _, n := range numbers {
		sum += n
	}
	return sum
}

// Clamp 将值限制在指定范围内
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Lerp 线性插值
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// RoundToPrecision 四舍五入到指定小数位
func RoundToPrecision(value float64, precision int) float64 {
	precisionMultiplier := math.Pow(10, float64(precision))
	return math.Round(value*precisionMultiplier) / precisionMultiplier
}
