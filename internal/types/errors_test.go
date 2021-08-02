package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateErrors(t *testing.T) {
	require.NotNil(t, NewPermissionError("mesg"))
	require.NotNil(t, NewQuotaExceededError("mesg"))
	require.NotNil(t, NewValidationError("mesg"))
	require.NotNil(t, NewDuplicateError("mesg"))
	require.NotNil(t, NewNotFoundError("mesg"))
	require.NotNil(t, NewJobRequeueError("mesg"))
	require.NotNil(t, NewFatalError("mesg"))
	require.NotNil(t, NewConflictError("mesg"))
}

func Test_ShouldGetSetInternal(t *testing.T) {
	err := NewPermissionError("mesg")
	require.Equal(t, "*types.BaseError: message=mesg", err.Error())
	require.Nil(t, err.Unwrap())
	err.SetInternal(fmt.Errorf("error"))
	require.NotNil(t, err.Unwrap())
	require.Equal(t, "*types.BaseError: message=mesg, internal=error", err.Error())
}
