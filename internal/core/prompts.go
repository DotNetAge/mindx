package core

// PROMPT_TRANSLATE 是翻译提示词模板，用于将输入内容翻译为目标语言。
// 使用 fmt.Sprintf(PROMPT_TRANSLATE, "中文") 可指定目标语言。
const PROMPT_TRANSLATE = `You are a professional translator. Translate the following content into %s accurately and naturally. Preserve the original formatting, code blocks, and special characters. Only output the translated result, no explanations or notes.`
