package common

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

// custom context key for storing the auth token
type AuthKey struct {
}

type AuthValue struct {
	Authorization string // original value from the request, only in SSE mode

	ServerMode string // stdio or sse

	Username string `envconfig:"USERNAME"`
	Password string `envconfig:"PASSWORD"`
}

// AuthForHTTP extracts the auth token from the request headers.
func AuthForHTTP(ctx context.Context, r *http.Request) context.Context {
	authVal := AuthValue{
		ServerMode: "sse",
	}

	authVal.Authorization = r.Header.Get("Authorization")

	if len(authVal.Authorization) > 0 {
		// we only support basic auth
		if isBasicAuth(authVal.Authorization) {
			// basic
			username, password := parseBasicAuth(authVal.Authorization)
			authVal.Username = username
			authVal.Password = password
		} else if isBearerAuth(authVal.Authorization) {
			// bearer
			username, password := parseBearerAuth(authVal.Authorization)
			authVal.Username = username
			authVal.Password = password
		}
	}

	if len(authVal.Username) == 0 {
		authVal.Username = "anonymous"
		authVal.Password = ""
	}

	return context.WithValue(ctx, AuthKey{}, authVal)
}

func isBasicAuth(authorization string) bool {
	return strings.HasPrefix(authorization, "Basic ")
}

func isBearerAuth(authorization string) bool {
	return strings.HasPrefix(authorization, "Bearer ")
}

func parseBasicAuth(authorization string) (string, string) {
	username := "anonymous"
	password := ""
	if strings.HasPrefix(authorization, "Basic ") {
		basicAuth := strings.TrimPrefix(authorization, "Basic ")
		if !strings.Contains(basicAuth, ":") {
			// possibly base64 encoded string
			decodedBasicAuth, err := base64.StdEncoding.DecodeString(basicAuth)
			if err == nil {
				basicAuth = string(decodedBasicAuth)
			}
		}

		authArr := strings.Split(basicAuth, ":")
		if len(authArr) > 0 {
			username = authArr[0]
		}

		if len(authArr) > 1 {
			password = authArr[1]
		}
	}

	return username, password
}

func parseBearerAuth(authorization string) (string, string) {
	username := "anonymous"
	password := ""
	if strings.HasPrefix(authorization, "Bearer ") {
		bearerAuthToken := strings.TrimPrefix(authorization, "Bearer ")
		if !strings.Contains(bearerAuthToken, ":") {
			// possibly base64 encoded string
			decodedBearerAuthToken, err := base64.StdEncoding.DecodeString(bearerAuthToken)
			if err == nil {
				bearerAuthToken = string(decodedBearerAuthToken)
			}
		}

		// we currently do not support bearer token
		// just handle like basic auth for now
		authArr := strings.Split(bearerAuthToken, ":")
		if len(authArr) > 0 {
			username = authArr[0]
		}

		if len(authArr) > 1 {
			password = authArr[1]
		}
	}

	return username, password
}

// AuthForStdio extracts the auth token from the environment
func AuthForStdio(ctx context.Context) context.Context {
	logger := log.WithFields(log.Fields{
		"package":  "mode",
		"function": "AuthForStdio",
	})

	authVal := AuthValue{
		ServerMode: "stdio",
	}

	err := envconfig.Process("", &authVal)
	if err != nil {
		logger.Errorf("failed to process environment variables: %v", err)
	}

	if len(authVal.Username) == 0 {
		authVal.Username = "anonymous"
		authVal.Password = ""
	}

	return context.WithValue(ctx, AuthKey{}, authVal)
}

func AuthForTest() context.Context {
	authVal := AuthValue{
		ServerMode: "stdio",
		Username:   "anonymous",
		Password:   "",
	}

	return context.WithValue(context.Background(), AuthKey{}, authVal)
}

func GetAuthValue(ctx context.Context) (AuthValue, error) {
	authVal, ok := ctx.Value(AuthKey{}).(AuthValue)
	if !ok {
		return AuthValue{}, xerrors.Errorf("failed to get auth value from context")
	}
	return authVal, nil
}
