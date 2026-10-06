// SPDX-License-Identifier: AGPL-3.0-or-later

package transfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DetectReportFiles_HTML(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "reports", "index.html")
	require.NoError(t, os.MkdirAll(filepath.Dir(htmlPath), 0755))
	require.NoError(t, os.WriteFile(htmlPath, []byte("<html><head><title>My Report</title></head><body></body></html>"), 0644))

	reports := detectReportFiles([]string{htmlPath})

	require.Len(t, reports, 1)
	// Path stored is the zip-relative path (leading / stripped), not the absolute disk path
	require.False(t, strings.HasPrefix(reports[0].Path, "/"), "stored path must not start with /")
	require.True(t, strings.HasSuffix(reports[0].Path, "reports/index.html"))
	require.Equal(t, "My Report", reports[0].Title)
	require.Equal(t, "text/html", reports[0].MIMEType)
}

func Test_DetectReportFiles_HTMLFallbackToBasename(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "summary.html")
	// HTML without <title> — falls back to filename
	require.NoError(t, os.WriteFile(htmlPath, []byte("<html><body><p>no title</p></body></html>"), 0644))

	reports := detectReportFiles([]string{htmlPath})

	require.Len(t, reports, 1)
	require.Equal(t, "summary.html", reports[0].Title)
}

func Test_DetectReportFiles_Markdown(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Hello\nworld"), 0644))

	reports := detectReportFiles([]string{mdPath})

	require.Len(t, reports, 1)
	require.Equal(t, "README.md", reports[0].Title)
	require.Equal(t, "text/markdown", reports[0].MIMEType)
}

func Test_DetectReportFiles_IgnoresNonReportFiles(t *testing.T) {
	reports := detectReportFiles([]string{
		"data/output.json",
		"logs/run.log",
		"images/chart.png",
	})
	require.Empty(t, reports)
}

func Test_DetectReportFiles_Mixed(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "report.html")
	mdPath := filepath.Join(dir, "notes.md")
	pngPath := filepath.Join(dir, "chart.png")
	require.NoError(t, os.WriteFile(htmlPath, []byte("<html><head><title>Dashboard</title></head></html>"), 0644))
	require.NoError(t, os.WriteFile(mdPath, []byte("# Notes"), 0644))
	require.NoError(t, os.WriteFile(pngPath, []byte("PNG"), 0644))

	reports := detectReportFiles([]string{htmlPath, mdPath, pngPath})

	require.Len(t, reports, 2)
	var titles []string
	for _, r := range reports {
		titles = append(titles, r.Title)
	}
	require.Contains(t, titles, "Dashboard")
	require.True(t, func() bool {
		for _, r := range reports {
			if strings.HasSuffix(r.Title, "notes.md") {
				return true
			}
		}
		return false
	}())
}

func Test_ExtractHTMLTitle_FallsBackToBasenameWhenNoTitle(t *testing.T) {
	html := strings.NewReader("<html><body><h1>Section One</h1></body></html>")
	title := extractHTMLTitle(html, "fallback.html")
	require.Equal(t, "fallback.html", title)
}

func Test_ExtractHTMLTitle_UsesTitle(t *testing.T) {
	html := strings.NewReader("<html><head><title>  Trimmed Title  </title></head></html>")
	title := extractHTMLTitle(html, "fallback.html")
	require.Equal(t, "Trimmed Title", title)
}

func Test_HTMLTitleFromFile_NonExistentFallsBackToBasename(t *testing.T) {
	title := htmlTitleFromFile("/nonexistent/path/my_report.html")
	require.Equal(t, "my_report.html", title)
}

func Test_DetectReportFiles_NormalizesAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "reports", "index.html")
	require.NoError(t, os.MkdirAll(filepath.Dir(htmlPath), 0755))
	require.NoError(t, os.WriteFile(htmlPath, []byte("<html><head><title>T</title></head></html>"), 0644))

	// Absolute disk path must be stored as relative zip path (leading / stripped)
	reports := detectReportFiles([]string{htmlPath})
	require.Len(t, reports, 1)
	require.False(t, strings.HasPrefix(reports[0].Path, "/"), "stored path must not start with /")
}
