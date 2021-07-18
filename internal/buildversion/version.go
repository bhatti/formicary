package buildversion

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
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
