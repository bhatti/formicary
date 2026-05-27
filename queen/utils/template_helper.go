package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
)

var markdownLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// UnescapeHTML flag
const UnescapeHTML = "UnescapeHTML"

// ParseTemplate parses GO template with dynamic parameters
func ParseTemplate(body string, data interface{}) (res string, err error) {
	if !strings.Contains(body, "{{") {
		return body, nil
	}
	emptyLineRegex, err := regexp.Compile(`(?m)^\s*$[\r\n]*|[\r\n]+\s+\z`)
	if err != nil {
		return "", err
	}
	t, err := template.New("").Funcs(TemplateFuncs()).Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse template due to %w", err)
	}
	var out bytes.Buffer
	err = t.Execute(&out, data)
	//err = t.ExecuteTemplate(&out, body, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template due to %w, data=%v", err, data)
	}
	res = emptyLineRegex.ReplaceAllString(out.String(), "")
	switch data.(type) {
	case map[string]interface{}:
		m := data.(map[string]interface{})
		if m[UnescapeHTML] == true {
			res = strings.ReplaceAll(res, "&lt;", "<")
		}
	}
	return
}

// TemplateFuncs returns template functions
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"Dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"Iterate": func(count uint) []uint {
			var i uint
			var Items []uint
			for i = 0; i < count; i++ {
				Items = append(Items, i)
			}
			return Items
		},
		"Add": func(n uint, plus uint) uint {
			return n + plus
		},
		"Unescape": func(s string) template.HTML {
			return template.HTML(s)
		},
		"Random": func(min, max int) int {
			return rand.Intn(max-min) + min
		},
		"MarkdownLink": func(s string) template.HTML {
			if s == "" {
				return ""
			}
			matches := markdownLinkRe.FindStringSubmatch(s)
			if matches == nil {
				return template.HTML(template.HTMLEscapeString(s))
			}
			text := template.HTMLEscapeString(matches[1])
			rawURL := matches[2]
			parsed, err := url.Parse(rawURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return template.HTML(template.HTMLEscapeString(s))
			}
			if strings.Contains(strings.ToLower(rawURL), "javascript:") {
				return template.HTML(template.HTMLEscapeString(s))
			}
			return template.HTML(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
				template.HTMLEscapeString(parsed.String()), text))
		},
	}
}
