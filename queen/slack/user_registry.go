// SPDX-License-Identifier: AGPL-3.0-or-later

package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	slackapi "github.com/slack-go/slack"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
)

const (
	configKeySlackUserID   = "slack_user_id"
	configKeySlackAPIToken = "slack_api_token"
)

// UserRegistry maps Slack user IDs to Formicary users.
// Mappings are stored in the existing user_configs table using the same AES-256-GCM
// encryption infrastructure used for all other user secrets.
type UserRegistry struct {
	cfg          *config.ServerConfig
	userManager  *manager.UserManager
	configRepo   repository.ConfigRepository
}

// NewUserRegistry constructs a UserRegistry.
func NewUserRegistry(
	cfg *config.ServerConfig,
	userManager *manager.UserManager,
	configRepo repository.ConfigRepository,
) *UserRegistry {
	return &UserRegistry{
		cfg:         cfg,
		userManager: userManager,
		configRepo:  configRepo,
	}
}

// LookupBySlackID finds the Formicary user and their decrypted API token for a given Slack user ID.
// Returns nil, "", nil when the Slack user is not registered.
func (r *UserRegistry) LookupBySlackID(_ context.Context, slackUserID string) (*common.User, string, error) {
	if slackUserID == "" {
		return nil, "", nil
	}

	// Use an admin context to search across all users for this Slack ID.
	adminQC := common.NewQueryContextFromIDs("", "").WithAdmin()

	configs, _, err := r.configRepo.Query(adminQC, map[string]interface{}{
		"name":  configKeySlackUserID,
		"value": slackUserID,
		"configurable_type": common.ConfigurableTypeUser,
	}, 0, 2, nil)
	if err != nil {
		return nil, "", fmt.Errorf("slack user lookup: %w", err)
	}
	if len(configs) == 0 {
		return nil, "", nil
	}

	cfg := configs[0]
	userID := cfg.ConfigurableID

	userQC := common.NewQueryContextFromIDs(userID, "")
	user, err := r.userManager.GetUser(userQC, userID)
	if err != nil {
		return nil, "", fmt.Errorf("slack user load(%s): %w", userID, err)
	}

	tokenConfigs, _, err := r.configRepo.QueryUserConfigs(userQC, userID, 0, 50)
	if err != nil {
		return nil, "", fmt.Errorf("slack token configs(%s): %w", userID, err)
	}
	var apiToken string
	for _, tc := range tokenConfigs {
		if tc.Name == configKeySlackAPIToken {
			apiToken = tc.Value
			break
		}
	}
	return user, apiToken, nil
}

// Register validates the Formicary API token DM'd by the user, stores the
// bidirectional mapping, then deletes the DM to avoid token persistence in chat.
func (r *UserRegistry) Register(
	ctx context.Context,
	slackUserID string,
	formicaryToken string,
	client *slackapi.Client,
	channelID string,
	msgTS string,
) (*common.User, error) {
	if slackUserID == "" {
		return nil, fmt.Errorf("slack user ID is required")
	}
	formicaryToken = strings.TrimSpace(formicaryToken)
	if formicaryToken == "" {
		return nil, fmt.Errorf("formicary token is required")
	}

	// Validate token by parsing the JWT — no HTTP round-trip needed.
	claims, err := web.ParseToken(formicaryToken, r.cfg.Common.Auth.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if claims.UserID == "" {
		return nil, fmt.Errorf("token contains no user ID")
	}

	userQC := common.NewQueryContextFromIDs(claims.UserID, claims.OrgID)
	user, err := r.userManager.GetUser(userQC, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Store the Slack user ID → Formicary user mapping (not secret).
	slackIDCfg, err := common.NewUserConfig(user.ID, configKeySlackUserID, slackUserID, false)
	if err != nil {
		return nil, fmt.Errorf("build slack_user_id config: %w", err)
	}
	if _, err = r.configRepo.Save(userQC, slackIDCfg); err != nil {
		return nil, fmt.Errorf("save slack_user_id: %w", err)
	}

	// Store the API token encrypted at rest (secret=true uses AES-256-GCM).
	tokenCfg, err := common.NewUserConfig(user.ID, configKeySlackAPIToken, formicaryToken, true)
	if err != nil {
		return nil, fmt.Errorf("build slack_api_token config: %w", err)
	}
	if _, err = r.configRepo.Save(userQC, tokenCfg); err != nil {
		return nil, fmt.Errorf("save slack_api_token: %w", err)
	}

	// Attempt to redact the token from the user's DM by overwriting the message text.
	// Slack bots cannot delete messages authored by users (only their own), so we
	// replace the content with a redacted placeholder instead.
	if client != nil && channelID != "" && msgTS != "" {
		_, _, _, err = client.UpdateMessage(channelID, msgTS,
			slackapi.MsgOptionText("[token registered and redacted]", false))
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "UserRegistry",
				"SlackUserID": slackUserID,
				"Error":       err,
			}).Warnf("failed to redact token DM — please delete the message manually")
		}
	}

	logrus.WithFields(logrus.Fields{
		"Component":   "UserRegistry",
		"SlackUserID": slackUserID,
		"UserID":      user.ID,
		"Username":    user.Username,
	}).Infof("registered Slack user")

	return user, nil
}

// RegisterByUserID stores only the Slack user ID → Formicary user mapping,
// without requiring or storing an API token. Used by the one-time code flow
// where the user is already identified by the code exchange.
func (r *UserRegistry) RegisterByUserID(
	_ context.Context,
	slackUserID string,
	user *common.User,
	client *slackapi.Client,
	channelID string,
	msgTS string,
) error {
	if slackUserID == "" {
		return fmt.Errorf("slack user ID is required")
	}
	if user == nil {
		return fmt.Errorf("user is required")
	}

	userQC := common.NewQueryContextFromIDs(user.ID, user.OrganizationID)

	slackIDCfg, err := common.NewUserConfig(user.ID, configKeySlackUserID, slackUserID, false)
	if err != nil {
		return fmt.Errorf("build slack_user_id config: %w", err)
	}
	if _, err = r.configRepo.Save(userQC, slackIDCfg); err != nil {
		return fmt.Errorf("save slack_user_id: %w", err)
	}

	// Overwrite the setup code DM with a confirmation so the code is no longer visible.
	if client != nil && channelID != "" && msgTS != "" {
		_, _, _, err = client.UpdateMessage(channelID, msgTS,
			slackapi.MsgOptionText("[registration code used — this message can be deleted]", false))
		if err != nil {
			logrus.WithField("SlackUserID", slackUserID).Debugf("[UserRegistry] could not overwrite setup message (expected if bot lacks message:write for user messages)")
		}
	}

	logrus.WithFields(logrus.Fields{
		"Component":   "UserRegistry",
		"SlackUserID": slackUserID,
		"UserID":      user.ID,
		"Username":    user.Username,
	}).Infof("registered Slack user via one-time code")

	return nil
}
