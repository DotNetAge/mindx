package conv

import (

	"image/color"
	"math"
	"strings"
	"unicode/utf8"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

// ═══════════════════════════════════════════════════════════
// 流光文字渲染器（对齐 Desktop .shimmer-text CSS 渐变动画）
//
// Desktop:
//   .shimmer-text {
//     background: linear-gradient(90deg,
//       var(--text-secondary) 0%,
//       var(--text-secondary) 35%,
//       var(--accent-cyan) 50%,
//       var(--text-secondary) 65%,
//       var(--text-secondary) 100%);
//     background-size: 200% 100%;
//     animation: shimmer-sweep 1.8s linear infinite;
//   }
//
// 终端模拟：
//   - 逐字符颜色插值：每个字符有 base（dim gray），扫描峰值经过时插值到 accent cyan
//   - Shimmer.Pos 从 0→1 循环（1.8s 完成一次），Tick 每 250ms 推进
//   - f(charPos, peakPos) = distance / (len/2) → 0~1 intensity
//     color = lerp(baseColor, accentColor, intensity)
//
// 与旧 Blink 的区别：
//   Blink 是符号整体二态闪烁（⏺ on/off），Shimmer 是文字逐字符颜色渐变扫描，
//   视觉上是一条青色光带从文字上扫过，与 Desktop shimmer-text 完全等价。
// ═══════════════════════════════════════════════════════════

// ShimmerDuration 流光一个完整扫描周期的时长（对齐 Desktop 1.8s）。
const ShimmerDuration = 1800 // ms

// Shimmer 结构：一次流光动画的状态。
// 每个 executing 节点对应一个 Shimmer；Tick 驱动 Pos 推进。
type Shimmer struct {
	Text string    // 被渲染的文案
	Pos  float64   // 0.0 ~ 1.0，当前扫描峰值位置（相对于 rune 数的比例）
	Len  int       // rune 数量（utf8.RuneCountInString）
}

// NewShimmer 创建并启动一个流光动画（Pos 初始 0）。
func NewShimmer(text string) *Shimmer {
	return &Shimmer{
		Text: text,
		Pos:  0,
		Len:  utf8.RuneCountInString(text),
	}
}

// Advance 推进一帧流光位置。
// tickMs 是两次 Tick 的间隔（conv 包的 tickInterval = 250ms）。
// 位置在 0.0~1.0 之间循环。
func (s *Shimmer) Advance(tickMs int) *Shimmer {
	if s.Len == 0 {
		return s
	}
	step := float64(tickMs) / float64(ShimmerDuration)
	next := s.Pos + step
	if next > 1.0 {
		next -= 1.0 // 循环
	}
	return &Shimmer{Text: s.Text, Pos: next, Len: s.Len}
}

// Render 输出流光字符串（每个字符独立上色）。
// 峰值中心在 s.Pos * s.Len，距离越近越接近 accent cyan。
func (s *Shimmer) Render() string {
	if s.Len == 0 {
		return ""
	}

	runes := []rune(s.Text)
	peak := s.Pos * float64(s.Len)
	var b strings.Builder

	for i, r := range runes {
		intensity := charIntensity(float64(i), peak, float64(s.Len))
		c := interpolateColor(style.ThemeDim, style.ThemeCyan, intensity)
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(r)))
	}
	return b.String()
}

// RenderBold 加粗流光（状态动词 / 思考中等需要强调的场合）。
func (s *Shimmer) RenderBold() string {
	if s.Len == 0 {
		return ""
	}
	runes := []rune(s.Text)
	peak := s.Pos * float64(s.Len)
	var b strings.Builder
	for i, r := range runes {
		intensity := charIntensity(float64(i), peak, float64(s.Len))
		c := interpolateColor(style.ThemeDim, style.ThemeCyan, intensity)
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
	}
	return b.String()
}

// charIntensity 计算某字符位置的流光强度（0.0 = 全 dim gray，1.0 = 全 accent cyan）。
//
// Desktop CSS 渐变的比例映射：
//   0%~35% = 全 text-secondary（dim gray）
//   35%~50% = 线性过渡到 accent-cyan（cyan）
//   50%~65% = 线性过渡回 text-secondary
//   65%~100% = 全 text-secondary
//
// 扫描峰值 peakPos 对应 CSS 渐变中 accent-cyan 的位置（50% 那一点）。
// 字符距 peakPos 的距离决定了 intensity。
func charIntensity(charPos, peakPos, totalLen float64) float64 {
	if totalLen <= 0 {
		return 0
	}

	// 字符相对于峰值的距离（归一化到 0~1）
	dist := math.Abs(charPos - peakPos)

	// 扫描带宽度：半宽 = totalLen * 0.15（CSS 35%~50% 是 15% 的带宽）
	// 在带宽内线性插值，带宽外为 0
	halfBand := totalLen * 0.15
	if dist >= halfBand {
		return 0
	}

	// 带内：center → edge 线性衰减
	return 1.0 - dist/halfBand
}

// ═══════════════════════════════════════════════════════════
// 颜色插值辅助
// ═══════════════════════════════════════════════════════════

// interpolateColor 线性插值两个 hex 颜色（0.0 → a, 1.0 → b）。
// lipgloss.Color 的底层是十六进制字符串，需要解析 RGB 分量。
func interpolateColor(a, b color.Color, t float64) color.Color {
	// color.RGBA.RGBA() 返回 0~0xFFFF 范围（uint32），归一化后插值
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()

	r := float64(ar>>8) + (float64(br>>8)-float64(ar>>8))*t
	g := float64(ag>>8) + (float64(bg>>8)-float64(ag>>8))*t
	bl := float64(ab>>8) + (float64(bb>>8)-float64(ab>>8))*t

	return color.RGBA{
		R: uint8(clampByte(int(math.Round(r)))),
		G: uint8(clampByte(int(math.Round(g)))),
		B: uint8(clampByte(int(math.Round(bl)))),
		A: 255,
	}
}


func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
