// SPDX-License-Identifier: AGPL-3.0-or-later

package manager

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var cssURLPattern = regexp.MustCompile(`url\(([^)]+)\)`)

// rewriteHTMLRefs parses src as HTML and rewrites relative src/href/action
// attributes and inline CSS url() values so they route back through the report
// endpoint (reportBase + "?file=<resolved-zip-path>").
//
// currentFile is the zip-relative path of the file being served (e.g.
// "reports/index.html"). Relative references are resolved against its directory.
//
// External references (http/https/data/mailto/#) are never rewritten.
// If parsing fails the original src bytes are returned unchanged.
func rewriteHTMLRefs(src []byte, reportBase, currentFile string) []byte {
	currentDir := path.Dir(currentFile)
	doc, err := html.Parse(bytes.NewReader(src))
	if err != nil {
		return src
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i, attr := range n.Attr {
				switch attr.Key {
				case "src", "href", "action":
					n.Attr[i].Val = rewriteRef(attr.Val, reportBase, currentDir)
				case "style":
					n.Attr[i].Val = rewriteCSSURLs(attr.Val, reportBase, currentDir)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return src
	}
	return buf.Bytes()
}

func rewriteRef(ref, reportBase, currentDir string) string {
	if isExternalRef(ref) {
		return ref
	}
	var zipPath string
	if strings.HasPrefix(ref, "/") {
		zipPath = strings.TrimPrefix(ref, "/")
	} else {
		zipPath = path.Join(currentDir, ref)
	}
	// Reject path traversal — leave unsafe refs as-is (they will 404 safely)
	if strings.HasPrefix(zipPath, "..") {
		return ref
	}
	return fmt.Sprintf("%s?file=%s", reportBase, zipPath)
}

func isExternalRef(ref string) bool {
	return ref == "" ||
		strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "//") ||
		strings.HasPrefix(ref, "#") ||
		strings.HasPrefix(ref, "data:") ||
		strings.HasPrefix(ref, "mailto:")
}

func rewriteCSSURLs(style, reportBase, currentDir string) string {
	return cssURLPattern.ReplaceAllStringFunc(style, func(m string) string {
		inner := m[4 : len(m)-1]
		inner = strings.Trim(inner, `"'`)
		rewritten := rewriteRef(inner, reportBase, currentDir)
		return "url(" + rewritten + ")"
	})
}
