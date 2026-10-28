package main

import "testing"

func TestClipboardCharacterCountsMatchWord(t *testing.T) {
	// 预期值取自 Word 的字符数统计，含标点，不将 CRLF 计为两个字符。
	tests := []struct {
		name, text    string
		with, without int
	}{
		{"空文本", "", 0, 0},
		{"中文英文和标点", "你好，world! 123", 13, 12},
		{"空格制表与换行", "a b\t c\r\nd", 7, 4},
		{"段落与分页", "a\r\nb\nc\rd\fe", 5, 5},
		{"全角和不换行空格", "a\u3000b\u00a0c", 5, 3},
		{"其他排版空格", "a\u2009b\u2005c", 5, 4},
		{"零宽字符", "a\u200bb\ufeffc", 3, 3},
		{"Emoji", "你好🙂👨‍👩‍👧‍👦", 10, 10},
		{"组合字", "e\u0301", 2, 2},
		{"只有空白", " \t\u3000\r\n", 3, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count := countCharacters(test.text)
			if !count.HasText || count.WithSpaces != test.with || count.WithoutSpaces != test.without {
				t.Fatalf("得到 %+v，预期计空格 %d，不计空格 %d", count, test.with, test.without)
			}
		})
	}
}
