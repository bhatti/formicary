package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
)

// ParseTemplate parses GO template with dynamic parameters
func ParseTemplate(body string, data interface{}) (string, error) {
	emptyLineRegex, err := regexp.Compile(`(?m)^\s*$[\r\n]*|[\r\n]+\s+\z`)
	if err != nil {
		return "", err
	}
	t, err := template.New("").Funcs(TemplateFuncs()).Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse template due to %s", err)
	}
	var out bytes.Buffer
	err = t.Execute(&out, data)
	//err = t.ExecuteTemplate(&out, body, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template due to %s", err)
	}
	return emptyLineRegex.ReplaceAllString(out.String(), ""), nil
}

// TemplateFuncs returns template functions
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
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
	}
}
