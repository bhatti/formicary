package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"plexobject.com/formicary/internal/auth"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/types/github"
)

// GithubAuth struct for oauth by GitHub
type GithubAuth struct {
	commonConfig *common.CommonConfig
	oauthConfig  *oauth2.Config
	callback     web.PostWebhookHandler
}

// NewGithubAuth constructor
func NewGithubAuth(
	commonConfig *common.CommonConfig,
	callback web.PostWebhookHandler) (auth.Provider, error) {
	callbackURL := fmt.Sprintf("%s/auth/github/callback",
		commonConfig.GetExternalBaseURL())

	if commonConfig.Auth.GoogleClientID == "" {
		return nil, fmt.Errorf("google oauth client-id is not specified")
	}
	if commonConfig.Auth.GoogleClientSecret == "" {
		return nil, fmt.Errorf("google oauth client-secret is not specified")
	}

	githubOauthConfig := &oauth2.Config{
		RedirectURL:  callbackURL,
		ClientID:     commonConfig.Auth.GithubClientID,
		ClientSecret: commonConfig.Auth.GithubClientSecret,
		Scopes:       []string{"public_repo", "read:user", "read:org", "user:email"}, // workflow repo
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}

	return &GithubAuth{
		commonConfig: commonConfig,
		oauthConfig:  githubOauthConfig,
		callback:     callback,
	}, nil
}

// AuthWebhookCallbackHandle callback handle
func (g *GithubAuth) AuthWebhookCallbackHandle(c web.WebContext) (err error) {
	defer func() {
		_ = c.Request().Body.Close()
	}()
	jobType := c.QueryParam("job")
	if jobType == "" {
		return fmt.Errorf("`job` query parameter not specified")
	}
	jobVersion := c.QueryParam("version")
	qc := web.BuildQueryContext(c)
	event, err := buildWebhookEvent(c)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "GithubAuth",
			"Event":     event,
			"Error":     err,
		}).Error("failed to handle webhook callback from git")
		return err
	}
	params := map[string]string{
		"GithubSHA256":        event.Sha256,
		"GithubPusher":        event.Pusher.Username,
		"GithubRepositoryURL": event.Repository.URL,
		"GitRepository":       event.Repository.Name,
		"GitCommitAuthor":     event.HeadCommit.Author.Username,
		"GitCommitID":         event.HeadCommit.ID,
		"GitCommitMessage":    event.HeadCommit.Message,
		"GitBranch":           event.Branch(),
	}

	return g.callback(
		qc,
		web.GetDBOrgFromSession(c),
		jobType,
		jobVersion,
		params,
		event.Sha256,
		event.Body,
	)
}

// AuthWebhookCallbackURL callback url
func (g *GithubAuth) AuthWebhookCallbackURL() string {
	return "/auth/github/webhook"
}

// AuthLoginURL login url
func (g *GithubAuth) AuthLoginURL() string {
	return "/auth/github"
}

// AuthLoginCallbackURL callback url
func (g *GithubAuth) AuthLoginCallbackURL() string {
	return "/auth/github/callback"
}

// AuthHandler returns url
func (g *GithubAuth) AuthHandler(state string) string {
	return g.oauthConfig.AuthCodeURL(state)
}

// String
func (g *GithubAuth) String() string {
	return fmt.Sprintf("Github Auth:%s:", g.AuthLoginURL())
}

// AuthUser - returns user info from GitHub response
func (g *GithubAuth) AuthUser(expectedState string, c web.WebContext) (*common.User, error) {
	return getUserInfoFromGithub(
		context.Background(),
		expectedState,
		c.FormValue("state"),
		c.FormValue("code"),
		g.oauthConfig)
}

func getUserInfoFromGithub(
	ctx context.Context,
	expectedState string,
	state string,
	code string,
	githubOauthConfig *oauth2.Config) (*common.User, error) {
	if state != expectedState {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := githubOauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	client := githubOauthConfig.Client(ctx, token)

	//response, err := client.Get("https://api.github.com/user/repos?page=0&per_page=100")
	response, err := client.Get("https://api.github.com/user?page=0&per_page=100")
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}

	defer func() {
		_ = response.Body.Close()
	}()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}

	mUser := make(map[string]interface{})
	err = json.Unmarshal(contents, &mUser)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s due to %s", string(contents), err.Error())
	}
	user := &common.User{
		Username:  mUser["login"].(string),
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if mUser["email"] != nil && reflect.TypeOf(mUser["email"]).String() == "string" {
		user.Email = mUser["email"].(string)
	}
	if mUser["name"] != nil && reflect.TypeOf(mUser["name"]).String() == "string" {
		user.Name = mUser["name"].(string)
	}
	if mUser["id"] != nil {
		user.AuthID = fmt.Sprintf("%v", mUser["id"])
	}
	if mUser["avatar_url"] != nil && reflect.TypeOf(mUser["avatar_url"]).String() == "string" {
		user.PictureURL = mUser["avatar_url"].(string)
	}
	if mUser["url"] != nil && reflect.TypeOf(mUser["url"]).String() == "string" {
		user.URL = mUser["url"].(string)
	}
	user.AuthProvider = "github"
	return user, nil
}

func buildWebhookEvent(
	c web.WebContext,
	) (event github.WebhookEvent, err error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return event, err
	}
	index := strings.Index(string(body), "payload=")
	if index == -1 {
		return event, fmt.Errorf("`payload` not specified in body")
	}
	payload := body[index+8:]
	decodedPayload, err := url.QueryUnescape(string(payload))
	if err != nil {
		return event, err
	}

	err = json.NewDecoder(bytes.NewReader([]byte(decodedPayload))).Decode(&event)
	if err != nil || event.Before == "" {
		return event, err
	}

	event.Headers = map[string]string{
		"User-Agent":          c.Request().Header["User-Agent"][0],
		"X-Forwarded-For":     c.Request().Header["X-Forwarded-For"][0],
		"X-Github-Delivery":   c.Request().Header["X-Github-Delivery"][0],
		"X-Github-Hook-Id":    c.Request().Header["X-Github-Hook-Id"][0],
		"X-Hub-Signature-256": c.Request().Header["X-Hub-Signature-256"][0],
	}
	if event.Pusher.Username == "" {
		event.Pusher.Username = event.Pusher.Login
	}
	if event.Sender.Username == "" {
		event.Sender.Username = event.Sender.Login
	}
	if event.Pusher.Username == "" {
		event.Pusher.Username = event.Sender.Username
	}
	if event.Repository.Owner.Username == "" {
		event.Repository.Owner.Username = event.Repository.Owner.Login
	}
	if event.HeadCommit.Author.Username == "" {
		event.HeadCommit.Author.Username = event.HeadCommit.Author.Login
	}
	if len(event.HeadCommit.Message) > 500 {
		event.HeadCommit.Message = event.HeadCommit.Message[0:500]
	}

	sha256Header := c.Request().Header["X-Hub-Signature-256"][0]
	sha256HashTokens := strings.SplitN(sha256Header, "=", 2)
	if len(sha256HashTokens) != 2 || sha256HashTokens[0] != "sha256" {
		return event, fmt.Errorf("failed to parse sha256 for %s", sha256Header)
	}
	event.Sha256 = sha256HashTokens[1]
	event.Body = body
	return
}
