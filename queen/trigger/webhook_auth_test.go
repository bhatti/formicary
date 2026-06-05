// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/queen/types"
)

func hmacSig(secret string, body []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write(body)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func Test_VerifyTriggerAuth_HMAC_Valid(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "mysecret"
	header := http.Header{"X-Hub-Signature-256": []string{hmacSig(secret, body)}}
	auth := &types.TriggerAuth{Method: "hmac_sha256", Header: "X-Hub-Signature-256"}
	require.NoError(t, verifyTriggerAuth(auth, secret, header, body))
}

func Test_VerifyTriggerAuth_HMAC_WithoutPrefix(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "mysecret"
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write(body)
	rawHex := hex.EncodeToString(h.Sum(nil))
	// Header without "sha256=" prefix — should still verify.
	header := http.Header{"X-Hub-Signature-256": []string{rawHex}}
	auth := &types.TriggerAuth{Method: "hmac_sha256", Header: "X-Hub-Signature-256"}
	require.NoError(t, verifyTriggerAuth(auth, secret, header, body))
}

func Test_VerifyTriggerAuth_HMAC_Invalid(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	header := http.Header{"X-Hub-Signature-256": []string{"sha256=badhash"}}
	auth := &types.TriggerAuth{Method: "hmac_sha256", Header: "X-Hub-Signature-256"}
	require.Error(t, verifyTriggerAuth(auth, "mysecret", header, body))
}

func Test_VerifyTriggerAuth_HMAC_MissingHeader(t *testing.T) {
	auth := &types.TriggerAuth{Method: "hmac_sha256", Header: "X-Hub-Signature-256"}
	require.Error(t, verifyTriggerAuth(auth, "mysecret", http.Header{}, []byte("body")))
}

func Test_VerifyTriggerAuth_Bearer_Valid(t *testing.T) {
	header := http.Header{"Authorization": []string{"Bearer secrettoken"}}
	auth := &types.TriggerAuth{Method: "bearer_token"}
	require.NoError(t, verifyTriggerAuth(auth, "secrettoken", header, nil))
}

func Test_VerifyTriggerAuth_Bearer_Invalid(t *testing.T) {
	header := http.Header{"Authorization": []string{"Bearer wrongtoken"}}
	auth := &types.TriggerAuth{Method: "bearer_token"}
	require.Error(t, verifyTriggerAuth(auth, "secrettoken", header, nil))
}

func Test_VerifyTriggerAuth_APIKey_Valid(t *testing.T) {
	header := http.Header{"X-Api-Key": []string{"myapikey"}}
	auth := &types.TriggerAuth{Method: "api_key_header", Header: "X-Api-Key"}
	require.NoError(t, verifyTriggerAuth(auth, "myapikey", header, nil))
}

func Test_VerifyTriggerAuth_APIKey_Invalid(t *testing.T) {
	header := http.Header{"X-Api-Key": []string{"wrongkey"}}
	auth := &types.TriggerAuth{Method: "api_key_header", Header: "X-Api-Key"}
	require.Error(t, verifyTriggerAuth(auth, "myapikey", header, nil))
}
