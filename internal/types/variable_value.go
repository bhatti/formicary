package types

import (
	"fmt"
)

// VariableValue defines structure for value of variable
type VariableValue struct {
	// Name defines name of property
	Name string `yaml:"name" json:"name"`
	// Value defines value of property
	Value interface{} `yaml:"value" json:"value"`
	// Secret for encryption
	Secret bool `yaml:"secret" json:"secret"`
	// ParsedValue defines value of property
	ParsedValue interface{} `yaml:"-" json:"-"`
}

// NewVariableValue creates new config property
func NewVariableValue(
	value interface{},
	secret bool) VariableValue {
	return VariableValue{Value: value, Secret: secret}
}

func (v VariableValue) String() string {
	return fmt.Sprintf("%s", v.Value)
}

// VariableValuesToMap converts a VariableValue map to a plain interface map.
// Callers must filter secrets first (e.g. via MaskVariableValues) before passing
// the result to template rendering or child job parameters.
func VariableValuesToMap(vars map[string]VariableValue) map[string]interface{} {
	m := make(map[string]interface{}, len(vars))
	for k, v := range vars {
		m[k] = v.Value
	}
	return m
}

// MaskVariableValues filers sensitive values
func MaskVariableValues(all map[string]VariableValue) (res map[string]VariableValue) {
	res = make(map[string]VariableValue)
	for k, v := range all {
		if !v.Secret {
			res[k] = v
		}
	}
	return
}

