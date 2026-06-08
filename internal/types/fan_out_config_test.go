// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFanOutConfig_Validate_Valid(t *testing.T) {
	f := &FanOutConfig{
		Source:          "regions",
		ItemVar:         "region",
		MaxParallel:     5,
		FailFast:        true,
		ExecutionMethod: Kubernetes,
	}
	require.NoError(t, f.Validate())
}

func TestFanOutConfig_Validate_NilIsOK(t *testing.T) {
	var f *FanOutConfig
	require.NoError(t, f.Validate())
}

func TestFanOutConfig_Validate_MissingSource(t *testing.T) {
	f := &FanOutConfig{ItemVar: "region"}
	err := f.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "source"))
}

func TestFanOutConfig_Validate_MissingItemVar(t *testing.T) {
	f := &FanOutConfig{Source: "regions"}
	err := f.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "item_var"))
}

func TestFanOutConfig_Validate_NegativeMaxParallel(t *testing.T) {
	f := &FanOutConfig{Source: "regions", ItemVar: "region", MaxParallel: -1}
	err := f.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "max_parallel"))
}

func TestFanOutConfig_Validate_ZeroMaxParallelIsUnlimited(t *testing.T) {
	f := &FanOutConfig{Source: "regions", ItemVar: "region", MaxParallel: 0}
	require.NoError(t, f.Validate())
}

func TestFanOutConfig_Validate_SourceTooLong(t *testing.T) {
	f := &FanOutConfig{Source: strings.Repeat("x", 201), ItemVar: "region"}
	err := f.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "source"))
}

func TestFanOutConfig_Validate_ItemVarTooLong(t *testing.T) {
	f := &FanOutConfig{Source: "regions", ItemVar: strings.Repeat("x", 101)}
	err := f.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "item_var"))
}
