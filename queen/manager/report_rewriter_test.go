// SPDX-License-Identifier: AGPL-3.0-or-later

package manager

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const reportBase = "/api/artifacts/abc123/download/raw"

func Test_RewriteHTMLRefs_RelativeRef(t *testing.T) {
	src := []byte(`<html><body><img src="images/chart.png"/></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	require.Contains(t, string(out), `src="/api/artifacts/abc123/download/raw?file=reports/images/chart.png"`)
}

func Test_RewriteHTMLRefs_AbsolutePathRef(t *testing.T) {
	src := []byte(`<html><body><a href="/styles/main.css">link</a></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	require.Contains(t, string(out), `href="/api/artifacts/abc123/download/raw?file=styles/main.css"`)
}

func Test_RewriteHTMLRefs_ExternalRefUnchanged(t *testing.T) {
	src := []byte(`<html><body><a href="https://example.com">ext</a></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	require.Contains(t, string(out), `href="https://example.com"`)
}

func Test_RewriteHTMLRefs_DataURIUnchanged(t *testing.T) {
	src := []byte(`<html><body><img src="data:image/png;base64,abc"/></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	require.Contains(t, string(out), `src="data:image/png;base64,abc"`)
}

func Test_RewriteHTMLRefs_HashRefUnchanged(t *testing.T) {
	src := []byte(`<html><body><a href="#section">anchor</a></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	require.Contains(t, string(out), `href="#section"`)
}

func Test_RewriteHTMLRefs_CSSURLInStyle(t *testing.T) {
	src := []byte(`<html><body><div style="background:url(img/bg.png)"></div></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	require.Contains(t, string(out), `url(/api/artifacts/abc123/download/raw?file=reports/img/bg.png)`)
}

func Test_RewriteHTMLRefs_PathTraversalLeftUnchanged(t *testing.T) {
	src := []byte(`<html><body><a href="../../../etc/passwd">bad</a></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/index.html")
	// traversal ref is left as the original value — never rewritten to a ?file= URL
	require.Contains(t, string(out), `href="../../../etc/passwd"`)
	require.NotContains(t, string(out), `?file=`)
}

func Test_RewriteHTMLRefs_NestedRelativePath(t *testing.T) {
	src := []byte(`<html><body><img src="../shared/logo.png"/></body></html>`)
	out := rewriteHTMLRefs(src, reportBase, "reports/sub/page.html")
	require.Contains(t, string(out), `src="/api/artifacts/abc123/download/raw?file=reports/shared/logo.png"`)
}

func Test_RewriteRef_EmptyRefUnchanged(t *testing.T) {
	result := rewriteRef("", reportBase, "reports")
	require.Equal(t, "", result)
}

func Test_RewriteCSSURLs_QuotedURL(t *testing.T) {
	result := rewriteCSSURLs(`background: url("img/foo.png")`, reportBase, "reports")
	require.True(t, strings.Contains(result, `url(/api/artifacts/abc123/download/raw?file=reports/img/foo.png)`))
}
