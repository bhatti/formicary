// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated Artifact.
// This file is NEVER overwritten by buf generate.

package resource

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	ulidpkg "github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ulid() string {
	entropy := ulidpkg.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) //nolint:gosec
	return ulidpkg.MustNew(ulidpkg.Timestamp(time.Now()), entropy).String()
}

func nowTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// TableName implements the GORM Tabler interface.
func (*Artifact) TableName() string { return "formicary_artifacts" }

// AfterLoad deserializes the metadata and tags JSON fields into their map representations.
func (a *Artifact) AfterLoad() error {
	if a.MetadataSerialized != "" {
		if err := json.Unmarshal([]byte(a.MetadataSerialized), &a.Metadata); err != nil {
			return fmt.Errorf("artifact AfterLoad metadata: %w", err)
		}
	}
	if a.TagsSerialized != "" {
		if err := json.Unmarshal([]byte(a.TagsSerialized), &a.Tags); err != nil {
			return fmt.Errorf("artifact AfterLoad tags: %w", err)
		}
	}
	return nil
}

// BeforeSave serializes the metadata and tags maps into their JSON column representations.
func (a *Artifact) BeforeSave() error {
	if len(a.Metadata) > 0 {
		b, err := json.Marshal(a.Metadata)
		if err != nil {
			return fmt.Errorf("artifact BeforeSave metadata: %w", err)
		}
		a.MetadataSerialized = string(b)
	}
	if len(a.Tags) > 0 {
		b, err := json.Marshal(a.Tags)
		if err != nil {
			return fmt.Errorf("artifact BeforeSave tags: %w", err)
		}
		a.TagsSerialized = string(b)
	}
	return nil
}

// Validate checks required fields on the artifact.
func (a *Artifact) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("name is not specified")
	}
	if a.Bucket == "" {
		return fmt.Errorf("bucket is not specified")
	}
	return nil
}

// ValidateBeforeSave serializes then validates the artifact.
func (a *Artifact) ValidateBeforeSave() error {
	if err := a.BeforeSave(); err != nil {
		return err
	}
	return a.Validate()
}
