// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated SystemConfig.
// This file is NEVER overwritten by buf generate.

package queen

import "fmt"

// TableName implements the GORM Tabler interface.
func (*SystemConfig) TableName() string { return "formicary_system_configs" }

// Validate checks required fields on the system config.
func (sc *SystemConfig) Validate() error {
	if sc.Name == "" {
		return fmt.Errorf("name is not specified")
	}
	if sc.Kind == "" {
		return fmt.Errorf("kind is not specified")
	}
	if sc.Value == "" {
		return fmt.Errorf("value is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the system config before persistence.
func (sc *SystemConfig) ValidateBeforeSave() error {
	return sc.Validate()
}

// Summary returns a short human-readable description of the system config.
func (sc *SystemConfig) Summary() string {
	return fmt.Sprintf("SystemConfig[scope=%s kind=%s name=%s]", sc.Scope, sc.Kind, sc.Name)
}
