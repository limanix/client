package config

import "testing"

func TestMarkdownEscaping(t *testing.T) {
	for _, test := range []struct {
		name, input, cell, code string
	}{
		{
			name:  "plain text",
			input: "tools and данные",
			cell:  "tools and данные",
			code:  "<code>tools and данные</code>",
		},
		{
			name:  "table separators and HTML",
			input: "<script>\"a|b\" & 'c'\r\n",
			cell:  "&lt;script&gt;&#34;a&#124;b&#34; &amp; &#39;c&#39;  ",
			code:  "<code>&lt;script&gt;&#34;a&#124;b&#34; &amp; &#39;c&#39;  </code>",
		},
		{
			name:  "code formatting",
			input: "`*_[]\\",
			cell:  "`*_[]\\",
			code:  "<code>&#96;&#42;&#95;&#91;&#93;&#92;</code>",
		},
		{
			name:  "literal entities",
			input: "&#96; &lt;b&gt;",
			cell:  "&amp;#96; &amp;lt;b&amp;gt;",
			code:  "<code>&amp;#96; &amp;lt;b&amp;gt;</code>",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := escapeMarkdownCell(test.input); got != test.cell {
				t.Errorf("cell = %q, want %q", got, test.cell)
			}
			if got := markdownCode(test.input); got != test.code {
				t.Errorf("code = %q, want %q", got, test.code)
			}
		})
	}
}
