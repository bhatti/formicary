// SPDX-License-Identifier: AGPL-3.0-or-later
// grpc-gateway mux factory: builds a REST-to-gRPC proxy with snake_case JSON.

package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// NewGatewayMux returns a grpc-gateway ServeMux configured for Formicary's conventions:
//
//   - JSON field names are proto field names (snake_case) — no camelCase conversion.
//   - Unknown JSON fields in requests are silently discarded.
//   - Unpopulated proto fields are omitted from responses.
//   - gRPC status errors are converted to HTTP status codes with a JSON error body.
//   - The "Authorization" HTTP header is forwarded to gRPC metadata.
func NewGatewayMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,  // snake_case field names
				EmitUnpopulated: false, // omit zero-value fields
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		runtime.WithIncomingHeaderMatcher(incomingHeaderMatcher),
		runtime.WithErrorHandler(errorHandler),
	)
}

// incomingHeaderMatcher forwards key HTTP headers to gRPC metadata.
// "cookie" is forwarded so that dashboard users authenticated via OAuth (session cookie)
// can call /api/v1/* without a separate Bearer token — the auth interceptor parses it.
func incomingHeaderMatcher(key string) (string, bool) {
	switch strings.ToLower(key) {
	case "authorization", "cookie", "x-request-id", "x-forwarded-for", "x-real-ip":
		return key, true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}

// errorHandler converts gRPC status errors to HTTP JSON responses that match
// the existing Formicary error format: {"error": "message", "code": "NOT_FOUND"}.
func errorHandler(_ context.Context, _ *runtime.ServeMux, _ runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	st, ok := status.FromError(err)
	if !ok {
		st = status.New(codes.Internal, err.Error())
	}

	type errorBody struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	body := errorBody{
		Error: st.Message(),
		Code:  st.Code().String(),
	}

	b, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		logrus.WithError(marshalErr).Warn("grpc-gateway: failed to marshal error response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(runtime.HTTPStatusFromCode(st.Code()))
	if _, writeErr := w.Write(b); writeErr != nil {
		logrus.WithError(writeErr).Warn("grpc-gateway: failed to write error response")
	}
}
