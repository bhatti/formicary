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
	t, err := template.New("").Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse template due to %s", err)
	}
	var out bytes.Buffer
	err = t.Execute(&out, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template due to %s", err)
	}
	return emptyLineRegex.ReplaceAllString(out.String(), ""), nil
}
