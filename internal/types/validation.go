package types

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

// ValidationResult contains the result of configuration validation
type ValidationResult struct {
	Valid    bool
	Warnings []string
	Errors   []string
	Info     []string
}

// AddWarning adds a warning message
func (vr *ValidationResult) AddWarning(msg string) {
	vr.Warnings = append(vr.Warnings, msg)
}

// AddError adds an error message and marks validation as failed
func (vr *ValidationResult) AddError(msg string) {
	vr.Errors = append(vr.Errors, msg)
	vr.Valid = false
}

// AddInfo adds an informational message
func (vr *ValidationResult) AddInfo(msg string) {
	vr.Info = append(vr.Info, msg)
}

// Log logs the validation result with appropriate log levels
func (vr *ValidationResult) Log() error {
	for _, info := range vr.Info {
		logrus.Debug(info)
	}

	for _, warning := range vr.Warnings {
		logrus.Warn(warning)
	}

	for _, err := range vr.Errors {
		logrus.Error(err)
	}

	if vr.Valid {
		return nil
	} else {
		return fmt.Errorf("validation failed: %v", vr.Errors)
	}
}
