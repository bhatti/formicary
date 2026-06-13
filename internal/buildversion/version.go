package buildversion

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"time"
)

// Info creates a formattable struct for output
type Info struct {
	Name    string    `json:"name,omitempty"`
	Version string    `json:"version,omitempty"`
	Commit  string    `json:"commit,omitempty"`
	Date    string    `json:"date,omitempty"`
	Started time.Time `json:"started,omitempty"`
}

// New will create a pointer to a new version object
func New(version string, commit string, date string, id string) *Info {
	return &Info{
		Name:    id,
		Version: version,
		Commit:  commit,
		Date:    date,
		Started: time.Now(),
	}
}

func (v *Info) String() string {
	if v.Version == "" || v.Version == v.Commit {
		return v.Commit
	}
	ver := "v" + strings.TrimPrefix(v.Version, "v")
	if v.Commit == "" {
		return ver
	}
	// Commit may be git-describe output (e.g. v0.1.67-1-g5c60db2-dirty)
	// or a bare short hash (e.g. abc1234 from Dockerfile).
	parts := strings.Split(v.Commit, "-")
	var hash, dirty string
	for _, p := range parts {
		if strings.HasPrefix(p, "g") && len(p) > 1 {
			hash = p // e.g. "g5c60db2"
		}
		if p == "dirty" {
			dirty = "-dirty"
		}
	}
	if hash != "" {
		ver += "-" + hash + dirty
	} else {
		// Bare short hash (no git-describe prefix): prepend g for clarity
		ver += "-g" + v.Commit + dirty
	}
	return ver
}

// Output will add the versioning code
func (v *Info) Output(shortened bool) string {
	var response string

	if shortened {
		response = v.ToShortened()
	} else {
		response = v.ToJSON()
	}
	return fmt.Sprintf("%s", response)
}

// ToJSON converts the Info into a JSON String
func (v *Info) ToJSON() string {
	bytes, _ := json.Marshal(v)
	return string(bytes) + "\n"
}

// ToShortened converts the Info into a JSON String
func (v *Info) ToShortened() string {
	bytes, _ := yaml.Marshal(v)
	return string(bytes) + "\n"
}
