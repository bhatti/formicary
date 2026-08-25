// SPDX-License-Identifier: AGPL-3.0-or-later

package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	queenconfig "plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	qtypes "plexobject.com/formicary/queen/types"
)

// slackRoutesConfigName is the SystemConfig name under which admins store the
// Slack route table as a JSON array of SlackRouteConfig objects.
// Managed via PUT /api/v1/configs (admin only). No code changes needed to add
// or update routes — the service reloads them each time it starts.
const slackRoutesConfigName = "SlackRoutes"

// slackSysconfigKind is the SystemConfig kind used to store admin-level Slack tokens.
// Stored as scope="default", kind="SLACK", name="BotToken" / "AppToken".
const slackSysconfigKind = "SLACK"

// SlackService manages the inbound Slack Socket Mode connection.
// It is a thin passthrough: it maps command verbs to Formicary job types,
// extracts trailing text and fixed params verbatim, and queues jobs.
// All domain logic lives in the job containers.
type SlackService struct {
	cfg              *config.ServerConfig
	jobManager       *manager.JobManager
	userManager      *manager.UserManager
	routerMu         sync.RWMutex
	router           *CommandRouter
	registry         *UserRegistry
	systemConfigRepo repository.SystemConfigRepository
	regCodeRepo      repository.SlackRegCodeRepository
	done             chan struct{}
	connected        atomic.Bool
	// Resolved tokens: env vars take priority; sysconfig is the fallback so
	// admins can rotate credentials via the API without redeploying the queen.
	botToken string
	appToken string
}

// IsConnected returns true after the Socket Mode connection receives a hello event.
func (s *SlackService) IsConnected() bool {
	return s.connected.Load()
}

// loadSlackTokensFromSysconfig returns bot and app tokens stored as admin
// SystemConfig entries (scope="default", kind="SLACK").  Returns empty strings
// when the entries are absent or the repository call fails.
func loadSlackTokensFromSysconfig(repo repository.SystemConfigRepository) (botToken, appToken string) {
	if bt, err := repo.GetByKindName(slackSysconfigKind, "BotToken"); err == nil && bt != nil {
		botToken = bt.Value
	}
	if at, err := repo.GetByKindName(slackSysconfigKind, "AppToken"); err == nil && at != nil {
		appToken = at.Value
	}
	return
}

// NewSlackService constructs a SlackService.
// Token resolution order: env vars (cfg.Slack) → SystemConfig admin entries.
// Returns nil, nil when Slack is disabled (no AppToken in either source).
func NewSlackService(
	cfg *config.ServerConfig,
	jobManager *manager.JobManager,
	userManager *manager.UserManager,
	configRepo repository.ConfigRepository,
	systemConfigRepo repository.SystemConfigRepository,
	regCodeRepo repository.SlackRegCodeRepository,
) (*SlackService, error) {
	botToken := cfg.Slack.BotToken
	appToken := cfg.Slack.AppToken

	// Fallback: load from admin SystemConfig when env vars are absent.
	if appToken == "" {
		sysBotToken, sysAppToken := loadSlackTokensFromSysconfig(systemConfigRepo)
		if sysAppToken != "" {
			appToken = sysAppToken
			logrus.WithField("Component", "SlackService").Info("Slack AppToken loaded from SystemConfig")
		}
		if botToken == "" && sysBotToken != "" {
			botToken = sysBotToken
			logrus.WithField("Component", "SlackService").Info("Slack BotToken loaded from SystemConfig")
		}
	}

	if appToken == "" {
		logrus.Warn("[SlackService] SLACK_APP_TOKEN not configured — Slack bot disabled. " +
			"Set env var SLACK_APP_TOKEN or add Admin > System Config (kind=SLACK name=AppToken)")
		return nil, nil
	}
	router := NewCommandRouter(cfg.Slack.Routes)
	registry := NewUserRegistry(cfg, userManager, configRepo)
	svc := &SlackService{
		cfg:              cfg,
		jobManager:       jobManager,
		userManager:      userManager,
		router:           router,
		registry:         registry,
		systemConfigRepo: systemConfigRepo,
		regCodeRepo:      regCodeRepo,
		done:             make(chan struct{}),
		botToken:         botToken,
		appToken:         appToken,
	}
	// Load admin-managed routes from SystemConfig on startup.
	// Errors are non-fatal — static routes from server config remain active.
	svc.reloadAdminRoutes()
	return svc, nil
}

// reloadAdminRoutes reads the SlackRoutes SystemConfig (admin scope) and merges
// those routes in front of any static routes from the server config YAML.
// Called at startup; can also be called by an admin-triggered reload endpoint.
func (s *SlackService) reloadAdminRoutes() {
	cfg, err := s.systemConfigRepo.GetByKindName("JSON", slackRoutesConfigName)
	if err != nil || cfg == nil {
		return // no admin routes configured — static routes only
	}
	var routes []queenconfig.SlackRouteConfig
	if err := json.Unmarshal([]byte(cfg.Value), &routes); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "SlackService",
			"Error":     err,
		}).Warnf("SlackRoutes SystemConfig is not valid JSON — ignoring")
		return
	}
	s.routerMu.Lock()
	s.router = s.router.WithOrgRoutes(routes)
	s.routerMu.Unlock()
	logrus.WithFields(logrus.Fields{
		"Component": "SlackService",
		"Count":     len(routes),
	}).Infof("loaded %d Slack routes from admin SystemConfig", len(routes))
}

// Start launches the Socket Mode event loop in a background goroutine.
// Non-blocking; ctx cancellation causes the loop to exit.
// On persistent connection_error the loop exits and is restarted with
// exponential backoff (10s → 20s → 40s … capped at 5 min).
func (s *SlackService) Start(ctx context.Context) error {
	if s.botToken == "" {
		logrus.Warn("[SlackService] SLACK_BOT_TOKEN not configured — outbound posting disabled")
	}
	logrus.WithFields(logrus.Fields{
		"Component":       "SlackService",
		"AppTokenPreview": tokenPreview(s.appToken),
		"BotTokenPreview": tokenPreview(s.botToken),
	}).Infof("Slack Socket Mode starting")

	go s.reconnectLoop(ctx)
	return nil
}

// reconnectLoop runs runEventLoop and restarts it with exponential backoff
// whenever it exits due to persistent connection errors.
func (s *SlackService) reconnectLoop(ctx context.Context) {
	backoff := 10 * time.Second
	const maxBackoff = 5 * time.Minute
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		default:
		}

		if attempt > 0 {
			logrus.WithFields(logrus.Fields{
				"Component": "SlackService",
				"Attempt":   attempt,
				"Backoff":   backoff,
			}).Warnf("Slack Socket Mode reconnecting after backoff")
			select {
			case <-ctx.Done():
				return
			case <-s.done:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
		attempt++

		loopCtx, cancel := context.WithCancel(ctx)
		api := slackapi.New(s.botToken, slackapi.OptionAppLevelToken(s.appToken))
		client := socketmode.New(api)
		done := s.runEventLoop(ctx, loopCtx, cancel, api, client)

		select {
		case <-ctx.Done():
			cancel()
			return
		case <-s.done:
			cancel()
			return
		case reconnect := <-done:
			cancel()
			if !reconnect {
				return // clean shutdown
			}
			// reconnect=true means connection_error — retry with backoff
		}
	}
}

// tokenPreview returns a masked preview of a Slack token for safe logging.
// Shows the type prefix (xapp-, xoxb-, etc.) and last 4 chars only.
func tokenPreview(token string) string {
	if token == "" {
		return "(empty)"
	}
	if len(token) <= 12 {
		return token[:4] + "****"
	}
	return token[:8] + "..." + token[len(token)-4:]
}

// Stop signals the event loop to exit and waits for it to finish.
func (s *SlackService) Stop() {
	close(s.done)
}

// runEventLoop runs the Socket Mode handler until loopCtx is cancelled, Stop
// is called, or too many consecutive connection_error events trigger a cancel.
// parentCtx is the queen's top-level context — used to distinguish clean
// shutdown (parentCtx.Err() != nil) from our own loopCtx cancels.
// Returns a channel that emits true when a reconnect should be attempted.
func (s *SlackService) runEventLoop(parentCtx, loopCtx context.Context, cancelCtx context.CancelFunc, api *slackapi.Client, client *socketmode.Client) <-chan bool {
	result := make(chan bool, 1)

	go func() {
		defer s.connected.Store(false)
		defer close(result)

		var connErrCount int32

		handler := socketmode.NewSocketmodeHandler(client)
		handler.Handle(socketmode.EventTypeConnected, func(evt *socketmode.Event, c *socketmode.Client) {
			atomic.StoreInt32(&connErrCount, 0) // reset on successful connect
			s.connected.Store(true)
			logrus.WithField("Component", "SlackService").Infof("Slack Socket Mode connected")
		})

		// Log connection_error with full payload so we can see the actual reason.
		// The data is *slack.ConnectionErrorEvent{Attempt, Backoff, ErrorObj}.
		handler.Handle(socketmode.EventTypeConnectionError, func(evt *socketmode.Event, c *socketmode.Client) {
			s.connected.Store(false)
			var errStr, backoffStr string
			var attempt int
			if ce, ok := evt.Data.(*slackapi.ConnectionErrorEvent); ok {
				if ce.ErrorObj != nil {
					errStr = ce.ErrorObj.Error()
				}
				backoffStr = ce.Backoff.String()
				attempt = ce.Attempt
			} else {
				raw, _ := json.Marshal(evt.Data)
				errStr = string(raw)
			}
			n := atomic.AddInt32(&connErrCount, 1)
			logrus.WithFields(logrus.Fields{
				"Component":       "SlackService",
				"AppTokenPreview": tokenPreview(s.appToken),
				"Error":           errStr,
				"SlackAttempt":    attempt,
				"SlackBackoff":    backoffStr,
				"ConsecutiveErrs": n,
			}).Errorf("Slack connection_error received")
			// After 3 consecutive errors without a successful connect, cancel the
			// loop so reconnectLoop can restart with backoff.
			if n >= 3 {
				logrus.WithField("Component", "SlackService").Warnf("3 consecutive connection_errors — will reconnect with backoff")
				cancelCtx()
			}
		})

		handler.HandleEvents(slackevents.AppMention, func(evt *socketmode.Event, c *socketmode.Client) {
			c.Ack(*evt.Request)
			outerEvt, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				logrus.WithFields(logrus.Fields{
					"Component": "SlackService",
					"DataType":  fmt.Sprintf("%T", evt.Data),
				}).Warnf("AppMention: unexpected evt.Data type (not EventsAPIEvent)")
				return
			}
			mention, ok := outerEvt.InnerEvent.Data.(*slackevents.AppMentionEvent)
			if !ok {
				logrus.WithFields(logrus.Fields{
					"Component":    "SlackService",
					"InnerType":    fmt.Sprintf("%T", outerEvt.InnerEvent.Data),
					"OuterEvtType": outerEvt.Type,
				}).Warnf("AppMention: unexpected InnerEvent type (not AppMentionEvent)")
				return
			}
			logrus.WithFields(logrus.Fields{
				"Component": "SlackService",
				"User":      mention.User,
				"Channel":   mention.Channel,
				"Text":      mention.Text,
			}).Infof("AppMention event received")
			s.handleAppMention(*mention, api)
		})
		handler.HandleEvents(slackevents.Message, func(evt *socketmode.Event, c *socketmode.Client) {
			c.Ack(*evt.Request)
			outerEvt, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				return
			}
			if msg, ok := outerEvt.InnerEvent.Data.(*slackevents.MessageEvent); ok {
				s.handleDirectMessage(*msg, api)
			}
		})
		handler.Handle(socketmode.EventTypeInteractive, func(evt *socketmode.Event, c *socketmode.Client) {
			c.Ack(*evt.Request)
			if cb, ok := evt.Data.(slackapi.InteractionCallback); ok {
				s.handleInteraction(cb, api)
			}
		})
		handler.Default = func(evt *socketmode.Event, c *socketmode.Client) {
			logrus.WithFields(logrus.Fields{
				"Component": "SlackService",
				"EventType": evt.Type,
				"DataType":  fmt.Sprintf("%T", evt.Data),
			}).Infof("SlackService: unhandled socket event")
		}

		runDone := make(chan error, 1)
		go func() { runDone <- handler.RunEventLoopContext(loopCtx) }()

		select {
		case <-s.done:
			logrus.WithField("Component", "SlackService").Infof("Slack Socket Mode stopped (Stop called)")
			result <- false
		case err := <-runDone:
			// Reconnect when the queen (parentCtx) is still alive.
			// loopCtx may have been cancelled by us (connection_error threshold),
			// but parentCtx.Err() is nil in that case — reconnect is correct.
			reconnect := parentCtx.Err() == nil
			if err != nil && err != context.Canceled {
				logrus.WithFields(logrus.Fields{
					"Component":       "SlackService",
					"AppTokenPreview": tokenPreview(s.appToken),
					"Error":           err,
					"WillReconnect":   reconnect,
				}).Errorf("Slack Socket Mode event loop exited with error")
			} else {
				logrus.WithFields(logrus.Fields{
					"Component":     "SlackService",
					"WillReconnect": reconnect,
				}).Warnf("Slack Socket Mode event loop exited")
			}
			result <- reconnect
		}
	}()

	return result
}

func (s *SlackService) handleAppMention(evt slackevents.AppMentionEvent, api *slackapi.Client) {
	if evt.BotID != "" {
		logrus.WithField("Component", "SlackService").Debugf("handleAppMention: ignoring bot message BotID=%s", evt.BotID)
		return
	}
	text := stripMention(evt.Text)
	logrus.WithFields(logrus.Fields{
		"Component": "SlackService",
		"User":      evt.User,
		"Channel":   evt.Channel,
		"RawText":   evt.Text,
		"Stripped":  text,
	}).Infof("handleAppMention: processing mention")
	channel := evt.Channel
	threadTS := evt.ThreadTimeStamp
	if threadTS == "" {
		threadTS = evt.TimeStamp
	}
	s.dispatch(context.Background(), evt.User, text, channel, threadTS, api)
}

func (s *SlackService) handleDirectMessage(evt slackevents.MessageEvent, api *slackapi.Client) {
	if evt.BotID != "" || evt.SubType != "" {
		return // skip bot messages and edits/deletes
	}
	text := strings.TrimSpace(evt.Text)
	channel := evt.Channel
	threadTS := evt.ThreadTimeStamp
	if threadTS == "" {
		threadTS = evt.TimeStamp
	}

	// DMs are used exclusively for the registration flow.
	// Format: "setup <token>"
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "setup ") {
		token := strings.TrimSpace(text[len("setup "):])
		s.handleSetup(context.Background(), evt.User, token, api, channel, evt.TimeStamp)
		return
	}
	// For non-DM channel messages that aren't app mentions, ignore.
}

func (s *SlackService) handleSetup(ctx context.Context, slackUserID, token string, api *slackapi.Client, channelID, msgTS string) {
	// Determine whether this is a one-time registration code or a legacy JWT token.
	// A registration code is 64 hex characters with no '.' separators.
	// A JWT has exactly two '.' separators (header.payload.signature).
	var apiToken string
	if s.regCodeRepo != nil && !strings.Contains(token, ".") && len(token) == 64 {
		// One-time code path: exchange code for the user's stored API token.
		regCode, err := s.regCodeRepo.Consume(token)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "SlackService",
				"SlackUserID": slackUserID,
				"Error":       err,
			}).Warnf("registration code exchange failed")
			_, _, _ = api.PostMessage(channelID,
				slackapi.MsgOptionText(fmt.Sprintf(
					"Registration failed: %s\nVisit %s/dashboard/slack/setup to generate a new code.",
					err.Error(), s.publicURL()), false))
			return
		}
		// Load the user identified by the registration code.
		qc := common.NewQueryContextFromIDs(regCode.UserID, regCode.OrgID)
		u, err := s.userManager.GetUser(qc, regCode.UserID)
		if err != nil {
			logrus.WithError(err).Warnf("[SlackService] code exchange: user not found")
			_, _, _ = api.PostMessage(channelID,
				slackapi.MsgOptionText("Registration failed: user not found.", false))
			return
		}
		// Register the Slack → user mapping without a token (no token to store).
		if err := s.registry.RegisterByUserID(ctx, slackUserID, u, api, channelID, msgTS); err != nil {
			logrus.WithError(err).Warnf("[SlackService] RegisterByUserID failed")
			_, _, _ = api.PostMessage(channelID,
				slackapi.MsgOptionText(fmt.Sprintf("Registration failed: %s", err.Error()), false))
			return
		}
		_, _, _ = api.PostMessage(channelID,
			slackapi.MsgOptionText(fmt.Sprintf(
				"Registered as *%s*. You can now use @bot commands in channels.",
				u.Username), false))
		return
	}

	// Legacy JWT path (backwards compat).
	if strings.Count(token, ".") == 2 {
		logrus.WithField("SlackUserID", slackUserID).
			Warn("[SlackService] setup via raw JWT token — consider using the dashboard setup code instead")
		apiToken = token
	} else {
		_, _, _ = api.PostMessage(channelID,
			slackapi.MsgOptionText(fmt.Sprintf(
				"Unrecognised setup code. Visit %s/dashboard/slack/setup to get a one-time registration code.",
				s.publicURL()), false))
		return
	}

	user, err := s.registry.Register(ctx, slackUserID, apiToken, api, channelID, msgTS)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":   "SlackService",
			"SlackUserID": slackUserID,
			"Error":       err,
		}).Warnf("registration failed")
		_, _, _ = api.PostMessage(channelID,
			slackapi.MsgOptionText(fmt.Sprintf("Registration failed: %s\nVisit %s/dashboard/slack/setup for a secure setup code.",
				err.Error(), s.publicURL()), false))
		return
	}
	_, _, _ = api.PostMessage(channelID,
		slackapi.MsgOptionText(fmt.Sprintf(
			"Registered as *%s*. You can now use @bot commands in channels.\n"+
				"_Consider using the dashboard setup page next time: %s/dashboard/slack/setup_",
			user.Username, s.publicURL()), false))
}

func (s *SlackService) handleInteraction(cb slackapi.InteractionCallback, api *slackapi.Client) {
	// Block Kit button interactions — future use.
	logrus.WithFields(logrus.Fields{
		"Component": "SlackService",
		"Type":      cb.Type,
	}).Debugf("interaction received (not yet handled)")
}

// dispatch routes a Slack text to a Formicary job and replies in thread.
func (s *SlackService) dispatch(ctx context.Context, slackUserID, text, channel, threadTS string, api *slackapi.Client) {
	text = strings.TrimSpace(text)
	logrus.WithFields(logrus.Fields{
		"Component": "SlackService",
		"UserID":    slackUserID,
		"Channel":   channel,
		"Text":      text,
		"ThreadTS":  threadTS,
	}).Infof("dispatching Slack command")

	// Builtin: setup instruction
	if strings.ToLower(text) == "setup" {
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf(
				"To register:\n1. Visit %s/dashboard/slack/setup\n2. Copy the one-time code shown\n3. DM me: `setup <code>`",
				s.publicURL()), false))
		return
	}

	// Builtin: help
	if strings.ToLower(text) == "help" {
		s.replyHelp(channel, threadTS, api)
		return
	}

	// Look up the registered user — required before submitting jobs.
	user, _, err := s.registry.LookupBySlackID(ctx, slackUserID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":   "SlackService",
			"SlackUserID": slackUserID,
			"Error":       err,
		}).Warnf("user lookup failed")
		return
	}
	if user == nil {
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf(
				"You're not registered yet. Visit %s/dashboard/slack/setup to get a one-time registration code, then DM me: `setup <code>`",
				s.publicURL()), false))
		return
	}

	// Check if this is a reply to a paused job thread — resume it.
	if threadTS != "" {
		if resumed := s.tryResumeJob(ctx, user, text, channel, threadTS, api); resumed {
			return
		}
	}

	s.routerMu.RLock()
	router := s.router
	s.routerMu.RUnlock()
	result, isBuiltin, err := router.Route(text)
	if err != nil {
		// Unknown command — route to ai-adhoc as a general ask so the user gets
		// an AI answer instead of a dead-end error message.
		s.submitAskJob(ctx, user, slackUserID, text, channel, threadTS, api)
		return
	}
	if isBuiltin {
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText("This command is handled by setup. Try `@bot setup` or `@bot help`", false))
		return
	}

	// Build and submit job — Formicary passes params verbatim; job container owns all AI logic.
	req := qtypes.NewRequest()
	req.JobType = result.JobType
	qc := common.NewQueryContextFromIDs(user.ID, user.OrganizationID)

	// addParam replies to the Slack thread and returns false on error.
	addParam := func(name, value string) bool {
		if _, e := req.AddParam(name, value); e != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "SlackService",
				"Param":     name,
				"Error":     e,
			}).Errorf("failed to add job param")
			_, _, _ = api.PostMessage(channel,
				slackapi.MsgOptionTS(threadTS),
				slackapi.MsgOptionText(fmt.Sprintf("Internal error preparing job: %s", e.Error()), false))
			return false
		}
		return true
	}

	if !addParam("SlackChannel", channel) {
		return
	}
	if !addParam("SlackThreadTs", threadTS) {
		return
	}
	if !addParam("SlackUserId", slackUserID) {
		return
	}

	// Strip Slack mrkdwn URL format <url|text> or <url> once; reuse for IdVar and description.
	cleanTrailing := ""
	if result.Trailing != "" {
		cleanTrailing = extractSlackURL(result.Trailing)
	}

	// Bind trailing text to the named IdVar (e.g. PRUrl, IssueNumber, Prompt).
	if result.IdVar != "" && cleanTrailing != "" {
		if !addParam(result.IdVar, cleanTrailing) {
			return
		}
	}

	// Merge fixed params from the route config verbatim — no interpretation.
	// These can be anything: Skill, Mode, Prompt templates, feature flags, etc.
	for k, v := range result.Params {
		if !addParam(k, v) {
			return
		}
	}

	if user.Username != "" && !addParam("UserTag", user.Username) {
		return
	}
	if tracker := s.defaultTracker(user.OrganizationID); tracker != "" && !addParam("DefaultTracker", tracker) {
		return
	}

	// Description: first 100 chars of trailing input (prompt/key), else the route's human label.
	if cleanTrailing != "" {
		if len(cleanTrailing) > 100 {
			req.Description = cleanTrailing[:100]
		} else {
			req.Description = cleanTrailing
		}
	} else if result.Description != "" {
		req.Description = result.Description
	}

	saved, err := s.jobManager.SaveJobRequest(qc, req)
	if err != nil {
		// Cron jobs have a user_key uniqueness constraint — a pending instance already
		// exists for today's schedule. Find it and trigger it immediately instead.
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "Duplicate entry") {
			if triggered, triggerErr := s.triggerExistingJobRequest(qc, result.JobType, req, channel, threadTS, api); triggered {
				if triggerErr != nil {
					_, _, _ = api.PostMessage(channel,
						slackapi.MsgOptionTS(threadTS),
						slackapi.MsgOptionText(fmt.Sprintf("Failed to trigger existing job: %s", triggerErr.Error()), false))
				}
				return
			}
		}
		logrus.WithFields(logrus.Fields{
			"Component": "SlackService",
			"JobType":   result.JobType,
			"UserID":    user.ID,
			"Error":     err,
		}).Warnf("failed to save job request")
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf("Failed to start job: %s", err.Error()), false))
		return
	}

	logrus.WithFields(logrus.Fields{
		"Component": "SlackService",
		"JobType":   result.JobType,
		"JobID":     saved.ID,
		"UserID":    user.ID,
	}).Infof("submitted job from Slack")

	link := fmt.Sprintf("%s/dashboard/jobs/requests/%s", s.publicURL(), saved.ID)
	_, _, _ = api.PostMessage(channel,
		slackapi.MsgOptionTS(threadTS),
		slackapi.MsgOptionText(fmt.Sprintf(
			"Started *%s* (job `%s`) — I'll post updates here. <%s|View>",
			result.JobType, saved.ShortID(), link), false))
}

// tryResumeJob checks for a PAUSED job in this thread and resumes it with the reply text.
func (s *SlackService) tryResumeJob(ctx context.Context, user *common.User, replyText, channel, threadTS string, api *slackapi.Client) bool {
	qc := common.NewQueryContextFromIDs(user.ID, user.OrganizationID)
	jobs, _, err := s.jobManager.QueryJobRequests(qc, map[string]interface{}{
		"job_state": "PAUSED",
		"user_id":   user.ID,
	}, 0, 10, []string{"created_at desc"})
	if err != nil || len(jobs) == 0 {
		return false
	}
	for _, job := range jobs {
		tsParam := job.GetParam("SlackThreadTs")
		if tsParam == nil || tsParam.Value != threadTS {
			continue
		}
		params := map[string]interface{}{
			"Prompt":    replyText,
			"ReplyText": replyText,
		}
		if err := s.jobManager.TriggerJobRequest(qc, job.ID, params, ""); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "SlackService",
				"JobID":     job.ID,
				"Error":     err,
			}).Warnf("failed to resume job")
			return false
		}
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf("Resuming job `%s`...", job.ShortID()), false))
		return true
	}
	return false
}

// submitAskJob routes unrecognized text to an ai-adhoc job with Skill=ygs-ask so
// Claude answers the question instead of showing a dead-end error message.
func (s *SlackService) submitAskJob(_ context.Context, user *common.User, slackUserID, text, channel, threadTS string, api *slackapi.Client) {
	qc := common.NewQueryContextFromIDs(user.ID, user.OrganizationID)
	req := qtypes.NewRequest()
	req.JobType = "ai-adhoc"
	req.Description = text
	if len([]rune(req.Description)) > 100 {
		req.Description = string([]rune(req.Description)[:100])
	}

	addOK := func(name, value string) bool {
		if _, e := req.AddParam(name, value); e != nil {
			_, _, _ = api.PostMessage(channel, slackapi.MsgOptionTS(threadTS),
				slackapi.MsgOptionText(fmt.Sprintf("Internal error: %s", e.Error()), false))
			return false
		}
		return true
	}

	if !addOK("SlackChannel", channel) {
		return
	}
	if !addOK("SlackThreadTs", threadTS) {
		return
	}
	if !addOK("SlackUserId", slackUserID) {
		return
	}
	if !addOK("Skill", "ygs-ask") {
		return
	}
	if !addOK("Prompt", text) {
		return
	}
	if user.Username != "" {
		addOK("UserTag", user.Username)
	}
	if tracker := s.defaultTracker(user.OrganizationID); tracker != "" {
		addOK("DefaultTracker", tracker)
	}

	saved, err := s.jobManager.SaveJobRequest(qc, req)
	if err != nil {
		_, _, _ = api.PostMessage(channel, slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf("Failed to submit ask: %s", err.Error()), false))
		return
	}
	link := fmt.Sprintf("%s/dashboard/jobs/requests/%s", s.publicURL(), saved.ID)
	_, _, _ = api.PostMessage(channel, slackapi.MsgOptionTS(threadTS),
		slackapi.MsgOptionText(fmt.Sprintf("Asking Claude... (job `%s`) <%s|View>", saved.ShortID(), link), false))
}

func (s *SlackService) replyHelp(channel, threadTS string, api *slackapi.Client) {
	lines := []string{
		"*Formicary Bot — Available Commands*",
		"",
		"*Setup & Registration*",
		fmt.Sprintf("• _Not registered?_ Go to <%s/dashboard/slack/setup|%s/dashboard/slack/setup>, copy the one-time code, then DM me:_ `setup <code>`", s.publicURL(), s.publicURL()),
		"",
		"*AI Workflows*",
	}
	s.routerMu.RLock()
	activeRoutes := s.router.Routes()
	s.routerMu.RUnlock()
	if len(activeRoutes) == 0 {
		lines = append(lines, "  _(no routes configured — ask your admin to set up Slack routes in the server config)_")
	} else {
		for _, route := range activeRoutes {
			if len(route.Triggers) == 0 {
				continue
			}
			desc := route.Description
			if desc == "" {
				desc = route.JobType
			}
			// Show primary trigger; list aliases if more than one.
			trigger := route.Triggers[0]
			if len(route.Triggers) > 1 {
				aliases := strings.Join(route.Triggers[1:], ", ")
				lines = append(lines, fmt.Sprintf("• `%s` _(also: %s)_ — %s", trigger, aliases, desc))
			} else {
				lines = append(lines, fmt.Sprintf("• `%s` — %s", trigger, desc))
			}
		}
	}
	lines = append(lines,
		"",
		"*Tips*",
		"• Commands with trailing text pass it as a _Prompt_ to the AI: `@bot adhoc explain this bug`",
		"• Reply in a paused job's thread to continue a human-in-the-loop workflow.",
		fmt.Sprintf("• View job status and history: %s/dashboard/jobs/requests", s.publicURL()),
		"",
		"• `help` — Show this message",
	)
	_, _, err := api.PostMessage(channel,
		slackapi.MsgOptionTS(threadTS),
		slackapi.MsgOptionText(strings.Join(lines, "\n"), false))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "SlackService",
			"Channel":   channel,
			"Error":     err,
		}).Errorf("replyHelp: PostMessage failed")
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "SlackService",
			"Channel":   channel,
		}).Infof("replyHelp: posted help message")
	}
}

func (s *SlackService) publicURL() string {
	url := s.cfg.Common.ExternalBaseURL
	if url == "" {
		port := s.cfg.Common.HTTPPort
		url = fmt.Sprintf("http://localhost:%d", port)
	}
	return strings.TrimRight(url, "/")
}

func (s *SlackService) defaultTracker(orgID string) string {
	if orgID == "" {
		return ""
	}
	configs, err := s.userManager.GetOrgConfigs(orgID)
	if err != nil {
		return ""
	}
	for _, c := range configs {
		if c.Name == "DefaultTracker" {
			return c.Value
		}
	}
	return ""
}

// extractSlackURL extracts a plain URL from Slack's mrkdwn link format.
// Slack wraps hyperlinks as <https://example.com|example.com> or <https://example.com>.
// Returns the input unchanged when it is not in that format.
func extractSlackURL(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		inner := s[1 : len(s)-1]
		if idx := strings.Index(inner, "|"); idx >= 0 {
			return inner[:idx]
		}
		return inner
	}
	return s
}

// triggerExistingJobRequest finds a cron-scheduled instance of jobType for the user's org
// and triggers it immediately. Mirrors the Python trigger_pending_or_submit logic:
//  1. WAITING (PENDING|PAUSED|READY) cron slots — trigger immediately
//  2. RUNNING  — already executing, just reply with a link
//  3. CANCELLED cron slots — re-activate via trigger endpoint
//
// Returns (true, err) if a candidate was found (even if trigger failed).
func (s *SlackService) triggerExistingJobRequest(
	qc *common.QueryContext,
	jobType string,
	req *qtypes.JobRequest,
	channel, threadTS string,
	api *slackapi.Client,
) (found bool, err error) {
	orgID := qc.GetOrganizationID()

	findCronJobs := func(state string) []*qtypes.JobRequest {
		jobs, _, e := s.jobManager.QueryJobRequests(qc.WithAdmin(), map[string]interface{}{
			"job_type":        jobType,
			"job_state":       state,
			"organization_id": orgID,
			"cron_triggered":  "1",
		}, 0, 5, []string{"scheduled_at asc"})
		if e != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "SlackService",
				"JobType":   jobType,
				"State":     state,
				"Error":     e,
			}).Debugf("triggerExistingJobRequest lookup")
		}
		return jobs
	}

	slackParams := map[string]interface{}{
		"SlackChannel":  channel,
		"SlackThreadTs": threadTS,
	}

	// 1. Pre-run waiting slot — trigger immediately.
	if jobs := findCronJobs("WAITING"); len(jobs) > 0 {
		target := jobs[0]
		link := fmt.Sprintf("%s/dashboard/jobs/requests/%s", s.publicURL(), target.GetID())
		if err = s.jobManager.TriggerJobRequest(qc.WithAdmin(), target.GetID(), slackParams, "triggered via Slack"); err != nil {
			return true, err
		}
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf(
				"Triggered *%s* (job `%s`) — I'll post updates here. <%s|View>",
				jobType, target.ShortID(), link), false))
		logrus.WithFields(logrus.Fields{
			"Component": "SlackService",
			"JobType":   jobType,
			"JobID":     target.GetID(),
		}).Infof("triggered waiting cron job from Slack")
		return true, nil
	}

	// 2. Already running — just reply with a link.
	if jobs := findCronJobs("RUNNING"); len(jobs) > 0 {
		target := jobs[0]
		link := fmt.Sprintf("%s/dashboard/jobs/requests/%s", s.publicURL(), target.GetID())
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf(
				"*%s* is already running (job `%s`). <%s|View>",
				jobType, target.ShortID(), link), false))
		return true, nil
	}

	// 3. Cancelled cron slot — re-activate it (trigger endpoint rotates user_key and sets state=PENDING).
	if jobs := findCronJobs("CANCELLED"); len(jobs) > 0 {
		target := jobs[0]
		link := fmt.Sprintf("%s/dashboard/jobs/requests/%s", s.publicURL(), target.GetID())
		logrus.WithFields(logrus.Fields{
			"Component": "SlackService",
			"JobType":   jobType,
			"JobID":     target.GetID(),
		}).Infof("re-activating cancelled cron slot from Slack")
		if err = s.jobManager.TriggerJobRequest(qc.WithAdmin(), target.GetID(), slackParams, "re-activated via Slack"); err != nil {
			return true, err
		}
		_, _, _ = api.PostMessage(channel,
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionText(fmt.Sprintf(
				"Triggered *%s* (job `%s`) — I'll post updates here. <%s|View>",
				jobType, target.ShortID(), link), false))
		return true, nil
	}

	return false, nil
}

// stripMention removes the leading <@UXXXXXXX> bot mention from text.
func stripMention(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<@") {
		if idx := strings.Index(text, ">"); idx != -1 {
			text = strings.TrimSpace(text[idx+1:])
		}
	}
	return text
}
