package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"plexobject.com/formicary/internal/auth"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// GoogleAuth for oauth support using Google login
type GoogleAuth struct {
	commonConfig *common.CommonConfig
	oauthConfig  *oauth2.Config
}

// AuthWebhookCallbackHandle returns webhook callback
func (g *GoogleAuth) AuthWebhookCallbackHandle(web.WebContext) error {
	return nil
}

// AuthWebhookCallbackURL returns url for webhook
func (g *GoogleAuth) AuthWebhookCallbackURL() string {
	return ""
}

// AuthLoginURL returns login url
func (g *GoogleAuth) AuthLoginURL() string {
	return "/auth/google"
}

// AuthLoginCallbackURL returns callback url
func (g *GoogleAuth) AuthLoginCallbackURL() string {
	return "/auth/google/callback"
}

// AuthHandler returns oauth response
func (g *GoogleAuth) AuthHandler(state string) string {
	return g.oauthConfig.AuthCodeURL(state)
}

// String
func (g *GoogleAuth) String() string {
	return fmt.Sprintf("Google Auth:%s:", g.AuthLoginURL())
}

// AuthUser builds user from oauth response
func (g *GoogleAuth) AuthUser(expectedState string, c web.WebContext) (*common.User, error) {
	return getUserInfoFromGoogle(
		context.Background(),
		expectedState,
		c.FormValue("state"),
		c.FormValue("code"),
		g.oauthConfig)
}

// NewGoogleAuth constructor
func NewGoogleAuth(
	commonConfig *common.CommonConfig) (auth.Provider, error) {
	callbackURL := fmt.Sprintf("%s/auth/google/callback", commonConfig.GetExternalBaseURL())
	if commonConfig.Auth.GoogleClientID == "" {
		return nil, fmt.Errorf("google oauth client-id is not specified")
	}
	if commonConfig.Auth.GoogleClientSecret == "" {
		return nil, fmt.Errorf("google oauth client-secret is not specified")
	}

	googleOauthConfig := &oauth2.Config{
		RedirectURL:  callbackURL,
		ClientID:     commonConfig.Auth.GoogleClientID,
		ClientSecret: commonConfig.Auth.GoogleClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	return &GoogleAuth{
		commonConfig: commonConfig,
		oauthConfig:  googleOauthConfig,
	}, nil
}

func getUserInfoFromGoogle(
	ctx context.Context,
	expectedState string,
	state string,
	code string,
	googleOauthConfig *oauth2.Config) (*common.User, error) {
	if state != expectedState {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := googleOauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
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
	if mUser["email"] == nil || reflect.TypeOf(mUser["email"]).String() != "string" {
		return nil, fmt.Errorf("failed to find email in  %s", string(contents))
	}
	user := &common.User{
		Username:  mUser["email"].(string),
		Email:     mUser["email"].(string),
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if mUser["id"] != nil && reflect.TypeOf(mUser["id"]).String() == "string" {
		user.AuthID = mUser["id"].(string)
	}
	if mUser["picture"] != nil && reflect.TypeOf(mUser["picture"]).String() == "string" {
		user.PictureURL = mUser["picture"].(string)
	}
	if mUser["verified_email"] != nil && mUser["verified_email"] == true {
		user.EmailVerified = true
	}
	user.AuthProvider = "google"
	return user, nil
}
