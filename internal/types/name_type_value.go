package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"plexobject.com/formicary/internal/crypto"
	"reflect"
	"strconv"
	"strings"
)

// local constants
const maxConfigValueLength = 10000
const encryptedPrefix = "_ENCRYPTED_"

// NameTypeValue defines structure for name, type, value
type NameTypeValue struct {
	// Name defines name of property
	Name string `yaml:"name" json:"name"`
	// Kind defines type of property value
	Kind string `yaml:"kind" json:"kind"`
	// Value defines value of property that can be string, number, boolean or JSON structure
	Value string `yaml:"value" json:"value"`
	// Secret for encryption
	Secret bool `yaml:"secret" json:"secret"`
	// ParsedValue defines value of property
	ParsedValue interface{} `yaml:"-" json:"-" gorm:"-"`
}

// NewNameTypeValue creates new config property
func NewNameTypeValue(
	name string,
	value interface{},
	secret bool) (NameTypeValue, error) {
	nv := NameTypeValue{Name: name, Secret: secret}
	if value == nil {
		return nv, fmt.Errorf("property for %v cannot be nil", name)
	}
	nv.ParsedValue = value
	nv.Kind = reflect.TypeOf(value).String()
	if nv.Kind == "string" {
		nv.Value = value.(string)
	} else {
		// JSON won't serialize map of interface to interface
		if reflect.TypeOf(value).String() == "map[interface {}]interface {}" {
			newValue := make(map[string]interface{})
			for k, v := range value.(map[interface{}]interface{}) {
				newValue[k.(string)] = v
			}
			value = newValue
		}
		b, err := json.Marshal(value)
		if err != nil {
			return nv, fmt.Errorf("failed to parse value for '%s' = '%v' due to %w",
				name, value, err)
		}
		nv.Value = string(b)
	}
	if len(nv.Value) > maxConfigValueLength {
		return nv, fmt.Errorf("value '%s' is too big", nv.Value)
	}
	return nv, nil
}

// Encrypt encrypts value
func (nv *NameTypeValue) Encrypt(key []byte) error {
	if len(key) > 0 && nv.Secret && nv.Value != "" && !strings.HasPrefix(nv.Value, encryptedPrefix) {
		b, err := crypto.Encrypt(key, []byte(nv.Value))
		if err == nil {
			nv.Value = encryptedPrefix + base64.StdEncoding.EncodeToString(b)
		} else {
			return err
		}
	}
	return nil
}

// Decrypt decrypts value
func (nv *NameTypeValue) Decrypt(key []byte) error {
	if len(key) > 0 && nv.Secret && nv.Value != "" && strings.HasPrefix(nv.Value, encryptedPrefix) {
		decodedString, err := base64.StdEncoding.DecodeString(nv.Value[len(encryptedPrefix):])
		if err == nil {
			b, err := crypto.Decrypt(key, decodedString)
			if err == nil {
				nv.Value = string(b)
			} else {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// GetVariableValue returns value
func (nv NameTypeValue) GetVariableValue() (val VariableValue, err error) {
	v, err := nv.GetParsedValue()
	if err != nil {
		return val, err
	}
	return NewVariableValue(v, nv.Secret), nil
}

// GetParsedValue parses value
func (nv NameTypeValue) GetParsedValue() (val interface{}, err error) {
	if nv.Kind == "string" {
		nv.ParsedValue = nv.Value
	} else if nv.Kind == "bool" {
		nv.ParsedValue = nv.Value == "true"
	} else if strings.HasPrefix(nv.Kind, "int") {
		var i int64
		if i, err = strconv.ParseInt(nv.Value, 10, 64); err != nil {
			return nil, err
		}
		nv.ParsedValue = i
	} else if strings.HasPrefix(nv.Kind, "float") {
		var f float64
		if f, err = strconv.ParseFloat(nv.Value, 64); err != nil {
			return nil, err
		}
		nv.ParsedValue = f
	} else if strings.HasPrefix(nv.Kind, "complex") {
		var c complex128
		if c, err = strconv.ParseComplex(nv.Value, 128); err != nil {
			return nil, err
		}
		nv.ParsedValue = c
	} else if strings.HasPrefix(nv.Value, "{") {
		m := make(map[string]interface{})
		err := json.Unmarshal([]byte(nv.Value), &m)
		if err != nil {
			return nil, err
		}
		nv.ParsedValue = m
	} else if strings.HasPrefix(nv.Value, "[") {
		arr := make([]interface{}, 0)
		err := json.Unmarshal([]byte(nv.Value), &arr)
		if err != nil {
			return nil, err
		}
		nv.ParsedValue = arr
	} else {
		return nil, fmt.Errorf(
			"failed to parse value for unsupported type '%v' for property '%v' with value '%v'",
			nv.Kind,
			nv.Name,
			nv.Value)
	}
	return nv.ParsedValue, nil
}

// IsNameRegular checks if name is not from artifact-url or forked-id
func (nv NameTypeValue) IsNameRegular() bool {
	return !nv.IsArtifactURL() && !nv.IsForkedJobID()
}

// IsArtifactURL checks if name is from artifact-url
func (nv NameTypeValue) IsArtifactURL() bool {
	return strings.Contains(nv.Name, "ArtifactURL")
}

// IsForkedJobID checks if name is for job-id
func (nv NameTypeValue) IsForkedJobID() bool {
	return strings.Contains(nv.Name, "ForkedJobID")
}

// IsPrimitiveType checks if type is builtin
func (nv NameTypeValue) IsPrimitiveType() bool {
	return nv.Kind == "bool" ||
		nv.Kind == "string" ||
		nv.Kind == "uint8" ||
		nv.Kind == "uint16" ||
		nv.Kind == "uint32" ||
		nv.Kind == "uint64" ||
		nv.Kind == "int8" ||
		nv.Kind == "int16" ||
		nv.Kind == "int32" ||
		nv.Kind == "int64" ||
		nv.Kind == "float32" ||
		nv.Kind == "float64" ||
		nv.Kind == "complex64" ||
		nv.Kind == "complex128" ||
		nv.Kind == "byte"
}

// MaskVariables filers sensitive values
func MaskVariables(all []NameTypeValue) (res []NameTypeValue) {
	res = make([]NameTypeValue, 0)
	for _, v := range all {
		if !v.Secret {
			res = append(res, v)
		}
	}
	return
}
