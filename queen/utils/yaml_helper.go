package utils

import (
	"fmt"
	"reflect"
	"strings"
)

func startIndexForNonWhitespace(line string) (startingIndex int, ch byte) {
	startingIndex = 0
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			startingIndex = i
			ch = line[i]
			break
		}
	}
	return
}

// ParseYamlTag finds yaml tag
func ParseYamlTag(input string, tag string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	parentOffsetMarker := -1
	offsetMarker := -1
	var currentBuf strings.Builder
	for n, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		if offsetMarker >= 0 {
			startingIndex, _ := startIndexForNonWhitespace(line)
			if startingIndex < offsetMarker || (line[startingIndex] != '-' && startingIndex <= parentOffsetMarker) {
				break
			}
			line = line[offsetMarker:]
			currentBuf.WriteString(line)
			currentBuf.WriteString("\n")
		} else {
			offsetMarker = strings.Index(line, tag)
			if offsetMarker >= 0 {
				var ch byte
				parentOffsetMarker, ch = startIndexForNonWhitespace(line)
				startingIndexVal := parentOffsetMarker
				if ch == '-' {
					line = strings.TrimSpace(line[startingIndexVal+1:])
					currentBuf.WriteString(line)
					currentBuf.WriteString("\n")
				}
				if n < len(lines)-1 {
					offsetMarker, _ = startIndexForNonWhitespace(lines[n+1])
				}
				if startingIndexVal == offsetMarker && len(line) > offsetMarker+len(tag)+1 {
					remaining := strings.TrimSpace(line[offsetMarker+len(tag):])
					if len(remaining) > 0 {
						currentBuf.WriteString(remaining)
					}
				}
			}
		}
	}
	return currentBuf.String()
}

// ParseNameValueConfigs parses name/value config
func ParseNameValueConfigs(input interface{}) (nameValueConfigs map[string]interface{}, err error) {
	nameValueConfigs = make(map[string]interface{})
	if input == nil {
		return nameValueConfigs, nil
	}
	switch t := input.(type) {
	case []interface{}:
		arr := input.([]interface{})
		for _, next := range arr {
			m := next.(map[interface{}]interface{})
			k := m["name"].(string)
			nameValueConfigs[k] = m["value"]
		}
	case map[interface{}]interface{}:
		m := input.(map[interface{}]interface{})
		for k, v := range m {
			nameValueConfigs[k.(string)] = v
		}
	case map[string]interface{}:
		nameValueConfigs = input.(map[string]interface{})
	default:
		err = fmt.Errorf("unknown type %s for config %v input",
			reflect.TypeOf(input), t)
	}
	return
}
