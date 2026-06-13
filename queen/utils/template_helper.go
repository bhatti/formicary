package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/sirupsen/logrus"
)

// JobTemplateHelper is the interface needed by template functions that query or
// submit jobs. States are passed as strings to avoid an import cycle with
// internal/types. It is satisfied by manager.JobManager.
type JobTemplateHelper interface {
	CountByJobTypeAndStateStrings(jobType string, states ...string) (int64, error)
	SubmitJob(jobType string, description string, params map[string]string) (string, error)
}

// JobCountQuerier is a backward-compatible alias for JobTemplateHelper.
// Deprecated: Use JobTemplateHelper instead.
type JobCountQuerier = JobTemplateHelper

var markdownLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// UnescapeHTML flag
const UnescapeHTML = "UnescapeHTML"

// ParseTemplate parses GO template with dynamic parameters
func ParseTemplate(body string, data interface{}) (res string, err error) {
	return ParseTemplateWithQuerier(body, data, nil)
}

// ParseTemplateWithQuerier parses a GO template with dynamic parameters and an optional
// job template helper that enables CountByJobTypeAndState and SubmitJob inside the template.
func ParseTemplateWithQuerier(body string, data interface{}, querier JobTemplateHelper) (res string, err error) {
	if !strings.Contains(body, "{{") {
		return body, nil
	}
	emptyLineRegex, err := regexp.Compile(`(?m)^\s*$[\r\n]*|[\r\n]+\s+\z`)
	if err != nil {
		return "", err
	}
	t, err := template.New("").Funcs(templateFuncs(querier)).Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse template due to %w", err)
	}
	var out bytes.Buffer
	err = t.Execute(&out, data)
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

// TemplateFuncs returns the full set of template functions.
// CountByJobTypeAndState returns 0 when no querier is available.
func TemplateFuncs() template.FuncMap {
	return templateFuncs(nil)
}

// templateFuncs builds a FuncMap with an optional helper for CountByJobTypeAndState and SubmitJob.
func templateFuncs(querier JobTemplateHelper) template.FuncMap {
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
		// String helpers used in trigger filter/param templates.
		"default": func(def interface{}, val interface{}) interface{} {
			if val == nil {
				return def
			}
			if s, ok := val.(string); ok && s == "" {
				return def
			}
			return val
		},
		"trimPrefix": func(s, prefix string) string {
			return strings.TrimPrefix(s, prefix)
		},
		"trimSuffix": func(s, suffix string) string {
			return strings.TrimSuffix(s, suffix)
		},
		"hasPrefix": func(s, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
		"hasSuffix": func(s, suffix string) bool {
			return strings.HasSuffix(s, suffix)
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"contains": strings.Contains,
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"Unescape": func(s string) htmltemplate.HTML {
			return htmltemplate.HTML(s)
		},
		"Random": func(min, max int) int {
			return rand.Intn(max-min) + min
		},
		"MarkdownLink": func(s string) htmltemplate.HTML {
			if s == "" {
				return ""
			}
			matches := markdownLinkRe.FindStringSubmatch(s)
			if matches == nil {
				if parsed, err := url.Parse(s); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
					escaped := htmltemplate.HTMLEscapeString(parsed.String())
					return htmltemplate.HTML(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, escaped, escaped))
				}
				return htmltemplate.HTML(htmltemplate.HTMLEscapeString(s))
			}
			text := htmltemplate.HTMLEscapeString(matches[1])
			rawURL := matches[2]
			parsed, err := url.Parse(rawURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return htmltemplate.HTML(htmltemplate.HTMLEscapeString(s))
			}
			if strings.Contains(strings.ToLower(rawURL), "javascript:") {
				return htmltemplate.HTML(htmltemplate.HTMLEscapeString(s))
			}
			return htmltemplate.HTML(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
				htmltemplate.HTMLEscapeString(parsed.String()), text))
		},
		// CountByJobTypeAndState counts job-requests matching a job-type and one or more states.
		// States may be individual strings or comma-separated (e.g. "PENDING,EXECUTING").
		// Returns 0 when no querier is provided or on error, so skip_if stays safe.
		//
		// Example skip_if:
		//   skip_if: >-
		//     {{if ge (CountByJobTypeAndState "my-job" "PENDING" "EXECUTING") 10}} true {{end}}
		"CountByJobTypeAndState": func(jobType string, stateArgs ...string) int64 {
			if querier == nil {
				return 0
			}
			states := make([]string, 0, len(stateArgs))
			for _, arg := range stateArgs {
				for _, part := range strings.Split(arg, ",") {
					if s := strings.TrimSpace(part); s != "" {
						states = append(states, s)
					}
				}
			}
			count, err := querier.CountByJobTypeAndStateStrings(jobType, states...)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "TemplateHelper",
					"JobType":   jobType,
					"States":    states,
				}).WithError(err).Warn("CountByJobTypeAndState query failed; returning 0")
			}
			return count
		},
		// SubmitJob submits a new job request and returns the job ID as a string.
		// params are key=value pairs passed as job parameters.
		// Returns "" on error or when no helper is available.
		//
		// Example:
		//   {{SubmitJob "resize-image" "Resize img-42" "ImageID=42" "Width=800"}}
		"SubmitJob": func(jobType string, description string, params ...string) string {
			if querier == nil {
				return ""
			}
			paramMap := parseKeyValueParams(params)
			id, err := querier.SubmitJob(jobType, description, paramMap)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":   "TemplateHelper",
					"JobType":     jobType,
					"Description": description,
				}).WithError(err).Warn("SubmitJob failed; returning empty string")
				return ""
			}
			return id
		},
		// SubmitJobsFromJSON submits one child job per item in a JSON array.
		// itemsJSON is a JSON array of objects — every field is passed as a job param.
		// Two reserved keys control behavior:
		//   "_description" — sets the job request description (optional)
		//   "_user_key"    — dedup key; prevents duplicate submissions on repeat runs
		// extraParams are additional "Key=Value" pairs merged into every submitted job.
		// Returns a newline-separated list of submitted job IDs (empty entry on failure).
		//
		// Example — submit one job per row returned by a query script:
		//   {{SubmitJobsFromJSON "process-order" .ItemsJSON "Region=us-east"}}
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"SubmitJobsFromJSON": func(jobType string, itemsJSON string, extraParams ...string) string {
			snip := itemsJSON
			if len(snip) > 200 {
				snip = snip[:200] + "..."
			}
			logrus.WithFields(logrus.Fields{
				"Component":    "TemplateHelper",
				"JobType":      jobType,
				"ItemsJSONLen": len(itemsJSON),
				"ItemsSnip":    snip,
			}).Infof("[SubmitJobsFromJSON] called")
			if querier == nil || itemsJSON == "" {
				logrus.WithFields(logrus.Fields{
					"Component":  "TemplateHelper",
					"JobType":    jobType,
					"QuerierNil": querier == nil,
				}).Warn("[SubmitJobsFromJSON] skipping: querier nil or empty itemsJSON")
				return ""
			}
			var items []map[string]interface{}
			if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "TemplateHelper",
					"JobType":   jobType,
					"ItemsJSON": itemsJSON,
				}).WithError(err).Warn("SubmitJobsFromJSON: failed to parse JSON")
				return ""
			}
			logrus.WithFields(logrus.Fields{
				"Component": "TemplateHelper",
				"JobType":   jobType,
				"ItemCount": len(items),
			}).Infof("[SubmitJobsFromJSON] submitting %d jobs", len(items))
			extra := parseKeyValueParams(extraParams)
			var ids []string
			for i, item := range items {
				// Build params from all fields in the item object.
				params := make(map[string]string, len(item)+len(extra))
				desc := ""
				for k, v := range item {
					str := fmt.Sprintf("%v", v)
					if k == "_description" {
						desc = str
					} else {
						params[k] = str
					}
				}
				// Extra params override item fields.
				for k, v := range extra {
					params[k] = v
				}
				// Default description from index if not provided by item.
				if desc == "" {
					desc = fmt.Sprintf("%s item %d", jobType, i+1)
				}
				id, err := querier.SubmitJob(jobType, desc, params)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "TemplateHelper",
						"JobType":   jobType,
						"ItemIndex": i,
					}).WithError(err).Warn("SubmitJobsFromJSON: SubmitJob failed")
					ids = append(ids, "")
				} else {
					logrus.WithFields(logrus.Fields{
						"Component":   "TemplateHelper",
						"JobType":     jobType,
						"ItemIndex":   i,
						"SubmittedID": id,
					}).Infof("[SubmitJobsFromJSON] submitted job id=%s", id)
					ids = append(ids, id)
				}
			}
			return strings.Join(ids, "\n")
		},
		// Percent100 returns the integer percentage of received out of required,
		// capped at 100, for use in progress bars.
		"Percent100": func(received, required int) int {
			if required <= 0 {
				return 100
			}
			p := received * 100 / required
			if p > 100 {
				return 100
			}
			return p
		},
		// FormatTime formats a time.Time for display; returns "—" for zero values.
		"FormatTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("2006-01-02 15:04:05")
		},
		// urlquery percent-encodes a string for safe inclusion in URL query parameters.
		"urlquery": url.QueryEscape,
	}
}

// parseKeyValueParams converts "Key=Value" strings into a map[string]string.
// Entries without "=" are silently skipped.
func parseKeyValueParams(pairs []string) map[string]string {
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		idx := strings.IndexByte(p, '=')
		if idx <= 0 {
			continue
		}
		m[p[:idx]] = p[idx+1:]
	}
	return m
}
