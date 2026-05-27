// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated ErrorCode.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"fmt"
	"regexp"
)

// TableName implements the GORM Tabler interface.
func (*ErrorCode) TableName() string { return "formicary_error_codes" }

// Editable returns true if the given user/org can edit this error code.
func (ec *ErrorCode) Editable(userID string, organizationID string) bool {
	if ec.OrganizationId != "" || organizationID != "" {
		return ec.OrganizationId == organizationID
	}
	return ec.UserId == userID
}

// ShortId returns the first 8 characters of the ID.
func (ec *ErrorCode) ShortId() string {
	if len(ec.Id) > 8 {
		return ec.Id[0:8] + "..."
	}
	return ec.Id
}

// Matches returns true if the given message matches this error code's regex pattern.
func (ec *ErrorCode) Matches(message string) bool {
	if message == "" || ec.Regex == "" {
		return false
	}
	match, err := regexp.MatchString(ec.Regex, message)
	return err == nil && match
}

// Validate checks required fields on the error code.
func (ec *ErrorCode) Validate() error {
	ec.Errors = make(map[string]string)
	var err error
	if ec.ErrorCode == "" {
		err = fmt.Errorf("errorCode is not specified")
		ec.Errors["ErrorCode"] = err.Error()
	}
	if ec.Regex == "" && ec.ExitCode == 0 {
		err = fmt.Errorf("regex or exitCode must be specified")
		ec.Errors["Regex"] = err.Error()
	}
	return err
}

// ValidateBeforeSave validates the error code before persistence.
func (ec *ErrorCode) ValidateBeforeSave() error {
	return ec.Validate()
}
