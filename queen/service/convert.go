// SPDX-License-Identifier: AGPL-3.0-or-later
// Type conversion helpers: old queen/types and internal/types → proto-generated types.
// The managers still operate on the old structs; these functions bridge the gap
// until Phase 6 migrates the full data layer to proto types.

package service

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonTypes "plexobject.com/formicary/internal/types"
	protoQueen "plexobject.com/formicary/gen/go/formicary/v1/queen"
	protoResource "plexobject.com/formicary/gen/go/formicary/v1/resource"
	protoUser "plexobject.com/formicary/gen/go/formicary/v1/user"
	queenTypes "plexobject.com/formicary/queen/types"
)

// ns converts a time.Duration to int64 nanoseconds for proto storage.
func ns(d time.Duration) int64 { return int64(d) }

// fromNs converts proto int64 nanoseconds back to time.Duration.
func fromNs(n int64) time.Duration { return time.Duration(n) }

// ---- JobDefinition --------------------------------------------------------

func toProtoJobDefinition(jd *queenTypes.JobDefinition) *protoQueen.JobDefinition {
	if jd == nil {
		return nil
	}
	p := &protoQueen.JobDefinition{
		Id:                    jd.ID,
		JobType:               jd.JobType,
		Version:               jd.Version,
		SemVersion:            jd.SemVersion,
		Url:                   jd.URL,
		UserId:                jd.UserID,
		OrganizationId:        jd.OrganizationID,
		Description:           jd.Description,
		Platform:              jd.Platform,
		NotifySerialized:      jd.NotifySerialized,
		CronTrigger:           jd.CronTrigger,
		TimeoutNs:             ns(jd.Timeout),
		PauseTimeNs:           ns(jd.PauseTime),
		Retry:                 int32(jd.Retry),
		HardResetAfterRetries: int32(jd.HardResetAfterRetries),
		DelayBetweenRetriesNs: ns(jd.DelayBetweenRetries),
		MaxConcurrency:        int32(jd.MaxConcurrency),
		Disabled:              jd.Disabled,
		PublicPlugin:          jd.PublicPlugin,
		RequiredParams:        jd.RequiredParams,
		UsesTemplate:          jd.UsesTemplate,
		DynamicTemplateTasks:  jd.DynamicTemplateTasks,
		Tags:                  jd.Tags,
		Methods:               jd.Methods,
		RawYaml:               jd.RawYaml,
		Active:                jd.Active,
		CanEdit:               jd.CanEdit,
		CreatedAt:             timestamppb.New(jd.CreatedAt),
		UpdatedAt:             timestamppb.New(jd.UpdatedAt),
	}
	for _, t := range jd.Tasks {
		p.Tasks = append(p.Tasks, toProtoTaskDefinition(t))
	}
	for _, c := range jd.Configs {
		p.Configs = append(p.Configs, toProtoJobDefinitionConfig(c))
	}
	for _, v := range jd.Variables {
		p.Variables = append(p.Variables, toProtoJobDefinitionVariable(v))
	}
	if jd.NameValueVariables != nil {
		if b, err := json.Marshal(jd.NameValueVariables); err == nil {
			p.NameValueVariablesJson = string(b)
		}
	}
	if len(jd.Notify) > 0 {
		if b, err := json.Marshal(jd.Notify); err == nil {
			p.NotifyJson = string(b)
		}
	}
	return p
}

func toProtoJobDefinitions(jds []*queenTypes.JobDefinition) []*protoQueen.JobDefinition {
	out := make([]*protoQueen.JobDefinition, 0, len(jds))
	for _, jd := range jds {
		out = append(out, toProtoJobDefinition(jd))
	}
	return out
}

func fromProtoJobDefinition(p *protoQueen.JobDefinition) *queenTypes.JobDefinition {
	if p == nil {
		return nil
	}
	jd := &queenTypes.JobDefinition{
		ID:                    p.Id,
		JobType:               p.JobType,
		Version:               p.Version,
		SemVersion:            p.SemVersion,
		URL:                   p.Url,
		UserID:                p.UserId,
		OrganizationID:        p.OrganizationId,
		Description:           p.Description,
		Platform:              p.Platform,
		NotifySerialized:      p.NotifySerialized,
		CronTrigger:           p.CronTrigger,
		Timeout:               fromNs(p.TimeoutNs),
		PauseTime:             fromNs(p.PauseTimeNs),
		Retry:                 int(p.Retry),
		HardResetAfterRetries: int(p.HardResetAfterRetries),
		DelayBetweenRetries:   fromNs(p.DelayBetweenRetriesNs),
		MaxConcurrency:        int(p.MaxConcurrency),
		Disabled:              p.Disabled,
		PublicPlugin:          p.PublicPlugin,
		RequiredParams:        p.RequiredParams,
		UsesTemplate:          p.UsesTemplate,
		DynamicTemplateTasks:  p.DynamicTemplateTasks,
		Tags:                  p.Tags,
		Methods:               p.Methods,
		RawYaml:               p.RawYaml,
	}
	for _, t := range p.Tasks {
		jd.Tasks = append(jd.Tasks, fromProtoTaskDefinition(t))
	}
	for _, c := range p.Configs {
		jd.Configs = append(jd.Configs, fromProtoJobDefinitionConfig(c))
	}
	for _, v := range p.Variables {
		jd.Variables = append(jd.Variables, fromProtoJobDefinitionVariable(v))
	}
	if p.NameValueVariablesJson != "" {
		_ = json.Unmarshal([]byte(p.NameValueVariablesJson), &jd.NameValueVariables)
	}
	if p.NotifyJson != "" {
		_ = json.Unmarshal([]byte(p.NotifyJson), &jd.Notify)
	}
	return jd
}

// ---- TaskDefinition -------------------------------------------------------

func toProtoTaskDefinition(t *queenTypes.TaskDefinition) *protoQueen.TaskDefinition {
	if t == nil {
		return nil
	}
	p := &protoQueen.TaskDefinition{
		Id:                    t.ID,
		JobDefinitionId:       t.JobDefinitionID,
		TaskType:              t.TaskType,
		Method:                string(t.Method),
		Description:           t.Description,
		HostNetwork:           t.HostNetwork,
		AllowFailure:          t.AllowFailure,
		AllowStartIfCompleted: t.AllowStartIfCompleted,
		AlwaysRun:             t.AlwaysRun,
		TimeoutNs:             ns(t.Timeout),
		Retry:                 int32(t.Retry),
		DelayBetweenRetriesNs: ns(t.DelayBetweenRetries),
		OnCompleted:           t.OnCompleted,
		OnFailed:              t.OnFailed,
		RequiredManualRoles:   t.RequiredManualRoles,
		TaskOrder:             int32(t.TaskOrder),
		ReportStdout:          t.ReportStdout,
		Headers:               t.Headers,
		BeforeScript:          t.BeforeScript,
		AfterScript:           t.AfterScript,
		Script:                t.Script,
		Except:                t.Except,
		CreatedAt:             timestamppb.New(t.CreatedAt),
		UpdatedAt:             timestamppb.New(t.UpdatedAt),
	}
	if t.OnExitCode != nil {
		if b, err := json.Marshal(t.OnExitCode); err == nil {
			p.OnExitCodeJson = string(b)
		}
	}
	for _, v := range t.Variables {
		p.Variables = append(p.Variables, toProtoTaskDefinitionVariable(v))
	}
	return p
}

func fromProtoTaskDefinition(p *protoQueen.TaskDefinition) *queenTypes.TaskDefinition {
	if p == nil {
		return nil
	}
	t := queenTypes.NewTaskDefinition(p.TaskType, commonTypes.TaskMethod(p.Method))
	t.ID = p.Id
	t.JobDefinitionID = p.JobDefinitionId
	t.Description = p.Description
	t.HostNetwork = p.HostNetwork
	t.AllowFailure = p.AllowFailure
	t.AllowStartIfCompleted = p.AllowStartIfCompleted
	t.AlwaysRun = p.AlwaysRun
	t.Timeout = fromNs(p.TimeoutNs)
	t.Retry = int(p.Retry)
	t.DelayBetweenRetries = fromNs(p.DelayBetweenRetriesNs)
	t.OnCompleted = p.OnCompleted
	t.OnFailed = p.OnFailed
	t.RequiredManualRoles = p.RequiredManualRoles
	t.TaskOrder = int(p.TaskOrder)
	t.ReportStdout = p.ReportStdout
	t.Headers = p.Headers
	t.BeforeScript = p.BeforeScript
	t.AfterScript = p.AfterScript
	t.Script = p.Script
	t.Except = p.Except
	if p.OnExitCodeJson != "" {
		_ = json.Unmarshal([]byte(p.OnExitCodeJson), &t.OnExitCode)
	}
	for _, v := range p.Variables {
		t.Variables = append(t.Variables, fromProtoTaskDefinitionVariable(v))
	}
	return t
}

// ---- JobDefinitionConfig --------------------------------------------------

func toProtoJobDefinitionConfig(c *queenTypes.JobDefinitionConfig) *protoQueen.JobDefinitionConfig {
	if c == nil {
		return nil
	}
	return &protoQueen.JobDefinitionConfig{
		Id:              c.ID,
		JobDefinitionId: c.JobDefinitionID,
		Name:            c.Name,
		Kind:            c.Kind,
		Value:           c.Value,
		Secret:          c.Secret,
		CreatedAt:       timestamppb.New(c.CreatedAt),
		UpdatedAt:       timestamppb.New(c.UpdatedAt),
	}
}

func fromProtoJobDefinitionConfig(p *protoQueen.JobDefinitionConfig) *queenTypes.JobDefinitionConfig {
	if p == nil {
		return nil
	}
	return &queenTypes.JobDefinitionConfig{
		ID:              p.Id,
		JobDefinitionID: p.JobDefinitionId,
		NameTypeValue: commonTypes.NameTypeValue{
			Name:   p.Name,
			Kind:   p.Kind,
			Value:  p.Value,
			Secret: p.Secret,
		},
	}
}

// ---- JobDefinitionVariable ------------------------------------------------

func toProtoJobDefinitionVariable(v *queenTypes.JobDefinitionVariable) *protoQueen.JobDefinitionVariable {
	if v == nil {
		return nil
	}
	return &protoQueen.JobDefinitionVariable{
		Id:              v.ID,
		JobDefinitionId: v.JobDefinitionID,
		Name:            v.Name,
		Kind:            v.Kind,
		Value:           v.Value,
		Secret:          v.Secret,
		CreatedAt:       timestamppb.New(v.CreatedAt),
		UpdatedAt:       timestamppb.New(v.UpdatedAt),
	}
}

func fromProtoJobDefinitionVariable(p *protoQueen.JobDefinitionVariable) *queenTypes.JobDefinitionVariable {
	if p == nil {
		return nil
	}
	return &queenTypes.JobDefinitionVariable{
		ID:              p.Id,
		JobDefinitionID: p.JobDefinitionId,
		NameTypeValue: commonTypes.NameTypeValue{
			Name:   p.Name,
			Kind:   p.Kind,
			Value:  p.Value,
			Secret: p.Secret,
		},
	}
}

// ---- TaskDefinitionVariable -----------------------------------------------

func toProtoTaskDefinitionVariable(v *queenTypes.TaskDefinitionVariable) *protoQueen.TaskDefinitionVariable {
	if v == nil {
		return nil
	}
	return &protoQueen.TaskDefinitionVariable{
		Id:               v.ID,
		TaskDefinitionId: v.TaskDefinitionID,
		Name:             v.Name,
		Kind:             v.Kind,
		Value:            v.Value,
		Secret:           v.Secret,
		CreatedAt:        timestamppb.New(v.CreatedAt),
		UpdatedAt:        timestamppb.New(v.UpdatedAt),
	}
}

func fromProtoTaskDefinitionVariable(p *protoQueen.TaskDefinitionVariable) *queenTypes.TaskDefinitionVariable {
	if p == nil {
		return nil
	}
	return &queenTypes.TaskDefinitionVariable{
		ID:               p.Id,
		TaskDefinitionID: p.TaskDefinitionId,
		NameTypeValue: commonTypes.NameTypeValue{
			Name:   p.Name,
			Kind:   p.Kind,
			Value:  p.Value,
			Secret: p.Secret,
		},
	}
}

// ---- JobRequest -----------------------------------------------------------

func toProtoJobRequest(jr *queenTypes.JobRequest) *protoQueen.JobRequest {
	if jr == nil {
		return nil
	}
	p := &protoQueen.JobRequest{
		Id:                 jr.ID,
		ParentId:           jr.ParentID,
		UserKey:            jr.UserKey,
		JobDefinitionId:    jr.JobDefinitionID,
		JobExecutionId:     jr.JobExecutionID,
		LastJobExecutionId: jr.LastJobExecutionID,
		OrganizationId:     jr.OrganizationID,
		UserId:             jr.UserID,
		Permissions:        int32(jr.Permissions),
		Description:        jr.Description,
		Platform:           jr.Platform,
		JobType:            jr.JobType,
		JobVersion:         jr.JobVersion,
		JobState:           string(jr.JobState),
		JobGroup:           jr.JobGroup,
		JobPriority:        int32(jr.JobPriority),
		TimeoutNs:          ns(jr.Timeout),
		ScheduleAttempts:   int32(jr.ScheduleAttempts),
		Retried:            int32(jr.Retried),
		CronTriggered:      jr.CronTriggered,
		QuickSearch:        jr.QuickSearch,
		ErrorCode:          jr.ErrorCode,
		ErrorMessage:       jr.ErrorMessage,
		CurrentTask:        jr.CurrentTask,
		ScheduledAt:        timestamppb.New(jr.ScheduledAt),
		CreatedAt:          timestamppb.New(jr.CreatedAt),
		UpdatedAt:          timestamppb.New(jr.UpdatedAt),
	}
	for _, param := range jr.Params {
		p.Params = append(p.Params, toProtoJobRequestParam(param))
	}
	if jr.NameValueParams != nil {
		if b, err := json.Marshal(jr.NameValueParams); err == nil {
			p.NameValueParamsJson = string(b)
		}
	}
	return p
}

// toProtoJobRequestSlice converts a slice of *queenTypes.JobRequest to proto.
func toProtoJobRequestSlice(reqs []*queenTypes.JobRequest) []*protoQueen.JobRequest {
	out := make([]*protoQueen.JobRequest, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, toProtoJobRequest(r))
	}
	return out
}

// ---- JobRequestParam ------------------------------------------------------

func toProtoJobRequestParam(p *queenTypes.JobRequestParam) *protoQueen.JobRequestParam {
	if p == nil {
		return nil
	}
	return &protoQueen.JobRequestParam{
		Id:           p.ID,
		JobRequestId: p.JobRequestID,
		Name:         p.Name,
		Value:        p.Value,
		Kind:         p.Kind,
		Secret:       p.Secret,
	}
}

// ---- User -----------------------------------------------------------------

func toProtoUser(u *commonTypes.User) *protoUser.User {
	if u == nil {
		return nil
	}
	return &protoUser.User{
		Id:               u.ID,
		Name:             u.Name,
		Username:         u.Username,
		Email:            u.Email,
		Url:              u.URL,
		PictureUrl:       u.PictureURL,
		OrganizationId:   u.OrganizationID,
		AuthId:           u.AuthID,
		AuthProvider:     u.AuthProvider,
		MaxConcurrency:   int32(u.MaxConcurrency),
		NotifySerialized: u.NotifySerialized,
		StickyMessage:    u.StickyMessage,
		BundleId:         u.BundleID,
		SerializedPerms:  u.SerializedPerms,
		SerializedRoles:  u.SerializedRoles,
		Salt:             u.Salt,
		EmailVerified:    u.EmailVerified,
		Locked:           u.Locked,
		Active:           u.Active,
		CreatedAt:        timestamppb.New(u.CreatedAt),
		UpdatedAt:        timestamppb.New(u.UpdatedAt),
	}
}

func toProtoUsers(users []*commonTypes.User) []*protoUser.User {
	out := make([]*protoUser.User, 0, len(users))
	for _, u := range users {
		out = append(out, toProtoUser(u))
	}
	return out
}

func fromProtoUser(p *protoUser.User) *commonTypes.User {
	if p == nil {
		return nil
	}
	u := commonTypes.NewUser(p.OrganizationId, p.Username, p.Name, p.Email, nil)
	u.ID = p.Id
	u.URL = p.Url
	u.PictureURL = p.PictureUrl
	u.AuthID = p.AuthId
	u.AuthProvider = p.AuthProvider
	u.MaxConcurrency = int(p.MaxConcurrency)
	u.NotifySerialized = p.NotifySerialized
	u.StickyMessage = p.StickyMessage
	u.BundleID = p.BundleId
	u.Salt = p.Salt
	u.EmailVerified = p.EmailVerified
	u.Locked = p.Locked
	u.Active = p.Active
	u.SerializedPerms = p.SerializedPerms
	u.SerializedRoles = p.SerializedRoles
	return u
}

// ---- JobExecution ---------------------------------------------------------

func toProtoJobExecution(je *queenTypes.JobExecution) *protoQueen.JobExecution {
	if je == nil {
		return nil
	}
	p := &protoQueen.JobExecution{
		Id:             je.ID,
		JobRequestId:   je.JobRequestID,
		JobType:        je.JobType,
		JobVersion:     je.JobVersion,
		JobState:       string(je.JobState),
		OrganizationId: je.OrganizationID,
		UserId:         je.UserID,
		CurrentTask:    je.CurrentTask,
		ExitCode:       je.ExitCode,
		ExitMessage:    je.ExitMessage,
		ErrorCode:      je.ErrorCode,
		ErrorMessage:   je.ErrorMessage,
		CpuSecs:        je.CPUSecs,
		Active:         je.Active,
		StartedAt:      timestamppb.New(je.StartedAt),
		UpdatedAt:      timestamppb.New(je.UpdatedAt),
	}
	if je.EndedAt != nil {
		p.EndedAt = timestamppb.New(*je.EndedAt)
	}
	for _, c := range je.Contexts {
		p.Contexts = append(p.Contexts, toProtoJobExecutionContext(c))
	}
	for _, t := range je.Tasks {
		p.Tasks = append(p.Tasks, toProtoTaskExecution(t))
	}
	return p
}

func toProtoJobExecutionContext(c *queenTypes.JobExecutionContext) *protoQueen.JobExecutionContext {
	if c == nil {
		return nil
	}
	return &protoQueen.JobExecutionContext{
		Id:             c.ID,
		JobExecutionId: c.JobExecutionID,
		Name:           c.Name,
		Value:          c.Value,
		Kind:           c.Kind,
		Secret:         c.Secret,
	}
}

func toProtoTaskExecution(t *queenTypes.TaskExecution) *protoQueen.TaskExecution {
	if t == nil {
		return nil
	}
	p := &protoQueen.TaskExecution{
		Id:               t.ID,
		JobExecutionId:   t.JobExecutionID,
		TaskType:         t.TaskType,
		Method:           string(t.Method),
		TaskState:        string(t.TaskState),
		AllowFailure:     t.AllowFailure,
		ExitCode:         t.ExitCode,
		ExitMessage:      t.ExitMessage,
		ErrorCode:        t.ErrorCode,
		ErrorMessage:     t.ErrorMessage,
		FailedCommand:    t.FailedCommand,
		Comments:         t.Comments,
		AntId:            t.AntID,
		AntHost:          t.AntHost,
		ManualReviewedBy: t.ManualReviewedBy,
		ReviewedStatus:   string(t.ReviewedStatus),
		Retried:          int32(t.Retried),
		TaskOrder:        int32(t.TaskOrder),
		CountServices:    int32(t.CountServices),
		CostFactor:       t.CostFactor,
		Stdout:           t.Stdout,
		Active:           t.Active,
		StartedAt:        timestamppb.New(t.StartedAt),
		UpdatedAt:        timestamppb.New(t.UpdatedAt),
	}
	if t.ManualReviewedAt != nil {
		p.ManualReviewedAt = timestamppb.New(*t.ManualReviewedAt)
	}
	if t.EndedAt != nil {
		p.EndedAt = timestamppb.New(*t.EndedAt)
	}
	for _, c := range t.Contexts {
		p.Contexts = append(p.Contexts, toProtoTaskExecutionContext(c))
	}
	for _, a := range t.Artifacts {
		p.Artifacts = append(p.Artifacts, toProtoArtifactFromCommon(a))
	}
	return p
}

func toProtoTaskExecutionContext(c *queenTypes.TaskExecutionContext) *protoQueen.TaskExecutionContext {
	if c == nil {
		return nil
	}
	return &protoQueen.TaskExecutionContext{
		Id:              c.ID,
		TaskExecutionId: c.TaskExecutionID,
		Name:            c.Name,
		Value:           c.Value,
		Kind:            c.Kind,
		Secret:          c.Secret,
	}
}

func toProtoArtifactFromCommon(a *commonTypes.Artifact) *protoResource.Artifact {
	if a == nil {
		return nil
	}
	p := &protoResource.Artifact{
		Id:                 a.ID,
		Bucket:             a.Bucket,
		Name:               a.Name,
		OrganizationId:     a.OrganizationID,
		UserId:             a.UserID,
		ArtifactGroup:      a.ArtifactGroup,
		Kind:               a.Kind,
		Etag:               a.ETag,
		ArtifactOrder:      int32(a.ArtifactOrder),
		JobRequestId:       a.JobRequestID,
		JobExecutionId:     a.JobExecutionID,
		TaskExecutionId:    a.TaskExecutionID,
		TaskType:           a.TaskType,
		Sha256:             a.SHA256,
		ContentType:        a.ContentType,
		ContentLength:      a.ContentLength,
		Permissions:        a.Permissions,
		MetadataSerialized: a.MetadataSerialized,
		TagsSerialized:     a.TagsSerialized,
		Active:             a.Active,
		Metadata:           a.Metadata,
		Tags:               a.Tags,
		Url:                a.URL,
		ExpiresAt:          timestamppb.New(a.ExpiresAt),
		CreatedAt:          timestamppb.New(a.CreatedAt),
		UpdatedAt:          timestamppb.New(a.UpdatedAt),
	}
	return p
}

// totalPages computes the total number of pages for pagination.
func totalPages(totalRecords int64, pageSize int32) int32 {
	if pageSize <= 0 {
		return 0
	}
	pages := totalRecords / int64(pageSize)
	if totalRecords%int64(pageSize) != 0 {
		pages++
	}
	return int32(pages)
}
