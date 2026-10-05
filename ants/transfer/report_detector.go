// SPDX-License-Identifier: AGPL-3.0-or-later

package transfer

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
	"plexobject.com/formicary/internal/types"
)

// reportFilePair associates a disk path (used for reading HTML title) with the
// zip-relative path (used as the stored ReportFile.Path that callers pass to
// ExtractFileFromArtifact). Both the local executor (ZipFiles) and the
// Kubernetes helper container executor must normalize zip entry names to the
// same relative format so that extraction lookups succeed.
type reportFilePair struct {
	diskPath string // absolute path on disk; used to open HTML files for title extraction
	zipPath  string // path as it will appear inside the archive (always relative, no leading /)
}

// detectReportFiles is a convenience wrapper for the local executor where the
// disk path can be normalized to the zip path by stripping the leading "/".
func detectReportFiles(diskPaths []string) []types.ReportFile {
	pairs := make([]reportFilePair, 0, len(diskPaths))
	for _, p := range diskPaths {
		clean := path.Clean(p)
		zipPath := strings.TrimPrefix(clean, "/")
		pairs = append(pairs, reportFilePair{diskPath: p, zipPath: zipPath})
	}
	return detectReportFilesFromPairs(pairs)
}

// detectReportFilesFromPairs inspects a list of (diskPath, zipPath) pairs and
// returns ReportFile metadata for every HTML/Markdown file. Called before
// zipping so files are still readable from diskPath.
func detectReportFilesFromPairs(pairs []reportFilePair) []types.ReportFile {
	var reports []types.ReportFile
	for _, pair := range pairs {
		lower := strings.ToLower(pair.diskPath)
		switch {
		case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
			title := htmlTitleFromFile(pair.diskPath)
			reports = append(reports, types.ReportFile{
				Path:     pair.zipPath,
				Title:    title,
				MIMEType: "text/html",
			})
		case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".markdown"):
			reports = append(reports, types.ReportFile{
				Path:     pair.zipPath,
				Title:    filepath.Base(pair.diskPath),
				MIMEType: "text/markdown",
			})
		}
	}
	return reports
}

// htmlTitleFromFile opens an HTML file from disk and extracts its <title> text.
// Falls back to the file's basename when the file cannot be opened or has no <title>.
func htmlTitleFromFile(diskPath string) string {
	f, err := os.Open(diskPath)
	if err != nil {
		return filepath.Base(diskPath)
	}
	defer func() { _ = f.Close() }()
	return extractHTMLTitle(f, diskPath)
}

// extractHTMLTitle parses an HTML reader and returns the <title> element text,
// falling back to the provided fallback basename.
func extractHTMLTitle(r io.Reader, fallback string) string {
	doc, err := html.Parse(r)
	if err != nil {
		return filepath.Base(fallback)
	}
	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "title" {
			if c := n.FirstChild; c != nil && c.Type == html.TextNode {
				if t := strings.TrimSpace(c.Data); t != "" {
					return t
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := walk(c); t != "" {
				return t
			}
		}
		return ""
	}
	if t := walk(doc); t != "" {
		return t
	}
	return filepath.Base(fallback)
}
