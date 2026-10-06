// SPDX-License-Identifier: AGPL-3.0-or-later

package manager

import (
	"bytes"
	"html"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var mdParser = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Table),
)

// renderMarkdownToHTML converts Markdown source bytes to a complete HTML page.
// title is used as the <title> element. The page uses a minimal CSS reset so
// reports render readably without external stylesheets.
func renderMarkdownToHTML(src []byte, title string) ([]byte, error) {
	var body bytes.Buffer
	if err := mdParser.Convert(src, &body); err != nil {
		return nil, err
	}
	return wrapHTMLFragment(title, body.Bytes()), nil
}

// wrapHTMLFragment wraps an HTML body fragment in a minimal full-page shell.
func wrapHTMLFragment(title string, body []byte) []byte {
	var b bytes.Buffer
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</title><style>`)
	b.WriteString(`body{font-family:system-ui,sans-serif;max-width:960px;margin:2rem auto;padding:0 1rem;line-height:1.6}`)
	b.WriteString(`pre,code{background:#f4f4f4;border-radius:4px;padding:.2em .4em}`)
	b.WriteString(`pre code{padding:0}pre{padding:1em;overflow-x:auto}`)
	b.WriteString(`table{border-collapse:collapse;width:100%}th,td{border:1px solid #ddd;padding:.5em .75em;text-align:left}`)
	b.WriteString(`th{background:#f0f0f0}`)
	b.WriteString(`</style></head><body>`)
	b.Write(body)
	b.WriteString(`</body></html>`)
	return b.Bytes()
}
