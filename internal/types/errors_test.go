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

func Test_SchedulingError_NoAnt_MapsToErrorNoAntForMethod(t *testing.T) {
	err := NewSchedulingError(SchedulingErrNoAnt, "no ant for method='%s'", "KUBERNETES")
	require.Equal(t, "no ant for method='KUBERNETES'", err.Error())
	require.Equal(t, ErrorNoAntForMethod, ErrorCodeForScheduling(err))
}

func Test_SchedulingError_AtCapacity_MapsToErrorAntResources(t *testing.T) {
	err := NewSchedulingError(SchedulingErrAtCapacity, "all ants at capacity")
	require.Equal(t, ErrorAntResources, ErrorCodeForScheduling(err))
}

func Test_ErrorCodeForScheduling_NonSchedulingError_FallsBackToAntResources(t *testing.T) {
	require.Equal(t, ErrorAntResources, ErrorCodeForScheduling(fmt.Errorf("some unrelated error")))
}
