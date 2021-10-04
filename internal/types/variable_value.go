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

