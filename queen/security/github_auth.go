package security

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/url"
	"plexobject.com/formicary/internal/auth"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/types/github"
	"reflect"
	"strings"
	"time"
)

// GithubAuth struct for oauth by github
type GithubAuth struct {
	commonConfig *common.CommonConfig
	oauthConfig  *oauth2.Config
	jobManager   *manager.JobManager
}

// NewGithubAuth constructor
func NewGithubAuth(
	commonConfig *common.CommonConfig,
	jobManager *manager.JobManager) (auth.Provider, error) {
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
		jobManager:   jobManager,
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
	qc := web.BuildQueryContext(c)
	jobDef, err := g.jobManager.GetJobDefinitionByType(qc, jobType)
	if err != nil {
		return err
	}
	jobReq, err := types.NewJobRequestFromDefinition(jobDef)
	if err != nil {
		return err
	}
	jobConfig := jobDef.GetConfig("GithubWebhookSecret")
	if jobConfig == nil || jobConfig.Value == "" {
		return fmt.Errorf("`GithubWebhookSecret` config is not set for job `%s`", jobType)
	}

	event, sha256Hash, err := buildWebhookEvent(c, jobConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":  "GithubAuth",
			"Event":      event,
			"JobRequest": jobReq,
			"Error":      err,
		}).Error("failed to handle webhook callback from git")
		return err
	}

	_, _ = jobReq.AddParam("GithubSHA256", sha256Hash)
	_, _ = jobReq.AddParam("GithubPusher", event.Pusher.Username)
	_, _ = jobReq.AddParam("GithubRepositoryURL", event.Repository.URL)
	_, _ = jobReq.AddParam("GitRepository", event.Repository.Name)
	_, _ = jobReq.AddParam("GitCommitAuthor", event.HeadCommit.Author.Username)
	_, _ = jobReq.AddParam("GitCommitID", event.HeadCommit.ID)
	_, _ = jobReq.AddParam("GitCommitMessage", event.HeadCommit.Message)
	_, _ = jobReq.AddParam("GitBranch", event.Branch())

	saved, err := g.jobManager.SaveJobRequest(qc, jobReq)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":  "GithubAuth",
			"Event":      event,
			"JobType":    jobType,
			"JobRequest": saved,
			"SHA256":     sha256Hash,
			"Error":      err,
		}).Errorf("failed to submit job request from github webhook")

		return err
	}

	logrus.WithFields(logrus.Fields{
		"Component":  "GithubAuth",
		"Event":      event,
		"JobType":    jobType,
		"JobRequest": saved,
		"User":       web.GetDBLoggedUserFromSession(c),
	}).Infof("submitted job as a result of github webhook")

	return nil
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

// AuthUser - returns user info from github response
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
	jobConfig *types.JobDefinitionConfig) (event github.WebhookEvent, hash256 string, err error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return event, "", err
	}
	index := strings.Index(string(body), "payload=")
	if index == -1 {
		return event, "", fmt.Errorf("`payload` not specified in body")
	}
	payload := body[index+8:]
	decodedPayload, err := url.QueryUnescape(string(payload))
	if err != nil {
		return event, "", err
	}

	err = json.NewDecoder(bytes.NewReader([]byte(decodedPayload))).Decode(&event)
	if err != nil || event.Before == "" {
		return event, "", err
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
		return event, "", fmt.Errorf("failed to parse sha256 for %s", sha256Header)
	}
	hash256 = sha256HashTokens[1]
	if err = verifySignature(jobConfig.Value, hash256, body); err != nil {
		return event, "", err
	}
	return
}

func verifySignature(secret string, expectedHash256 string, body []byte) error {
	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := hash.Write(body); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash256 {
		return fmt.Errorf("failed to match '%s' sha256 with '%s'", actualHash, expectedHash256)
	}
	return nil
}
