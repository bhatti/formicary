// SPDX-License-Identifier: AGPL-3.0-or-later

package manager

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_RenderMarkdownToHTML_BasicMarkdown(t *testing.T) {
	src := []byte("# Hello\n\nThis is a **paragraph** with `code`.\n")
	out, err := renderMarkdownToHTML(src, "Test Page")
	require.NoError(t, err)

	html := string(out)
	require.Contains(t, html, `<!DOCTYPE html>`)
	require.Contains(t, html, `<title>Test Page</title>`)
	require.Contains(t, html, `<h1>`)
	require.Contains(t, html, `Hello`)
	require.Contains(t, html, `<strong>`)
	require.Contains(t, html, `<code>`)
}

func Test_RenderMarkdownToHTML_GFMTable(t *testing.T) {
	src := []byte("| A | B |\n|---|---|\n| 1 | 2 |\n")
	out, err := renderMarkdownToHTML(src, "Table Test")
	require.NoError(t, err)
	require.Contains(t, string(out), `<table>`)
}

func Test_RenderMarkdownToHTML_TitleEscaped(t *testing.T) {
	src := []byte("hello")
	out, err := renderMarkdownToHTML(src, "<script>alert(1)</script>")
	require.NoError(t, err)
	html := string(out)
	require.NotContains(t, html, `<script>alert(1)</script>`)
	require.Contains(t, html, `&lt;script&gt;`)
}

func Test_WrapHTMLFragment_ProducesValidShell(t *testing.T) {
	body := []byte("<p>content</p>")
	out := wrapHTMLFragment("My Title", body)
	html := string(out)
	require.True(t, strings.HasPrefix(html, `<!DOCTYPE html>`))
	require.Contains(t, html, `<title>My Title</title>`)
	require.Contains(t, html, `<p>content</p>`)
	require.Contains(t, html, `</body></html>`)
}
