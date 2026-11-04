package trylet

// ClipboardCount 是剪贴板文本的字符统计结果。
type ClipboardCount struct {
	HasText       bool   `json:"hasText"`
	WithSpaces    int    `json:"withSpaces"`
	WithoutSpaces int    `json:"withoutSpaces"`
	Error         string `json:"error,omitempty"`
}

// CountCharacters 按 Word 的纯文本统计口径计算字符数。
func CountCharacters(text string) ClipboardCount {
	count := ClipboardCount{HasText: true}
	for _, character := range text {
		// 按 Word 的纯文本统计口径，段落、分页和零宽空格不计为字符。
		switch character {
		case '\r', '\n', '\f', '\u200b', '\ufeff':
			continue
		}
		count.WithSpaces++
		switch character {
		case ' ', '\t', '\v', '\u00a0', '\u2005', '\u3000':
			continue
		}
		count.WithoutSpaces++
	}
	return count
}
