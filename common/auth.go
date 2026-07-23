package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
)

// custom context key for storing the auth token
type AuthKey struct {
}

type ServerMode string

const (
	ServerModeSTDIO ServerMode = "stdio"
	ServerModeHTTP  ServerMode = "http"
)

type AuthValue struct {
	Authorization string // original value from the request, only in http mode

	ServerMode ServerMode

	Username string
	Password string
}

func NewAuthValueForHTTP(header http.Header) AuthValue {
	authVal := AuthValue{
		Authorization: header.Get("Authorization"),
		ServerMode:    ServerModeHTTP,
	}

	username := ""
	password := ""

	if authVal.IsBasicAuth() {
		username, password = authVal.parseBasicAuth()
		authVal.Username = username
		authVal.Password = password
	} else if authVal.IsBearerAuth() {
		username = authVal.parseUsernameFromJWT()
		if len(username) == 0 {
			username = header.Get("X-Forwarded-User")
			header.Del("X-Forwarded-User")
		}
		authVal.Username = username
	}

	// if authorization is not provided, use anonymous
	if len(username) == 0 {
		authVal.Username = "anonymous"
		authVal.Password = ""
	}

	return authVal
}

func NewAuthValueForSTDIO(config *Config) AuthValue {
	authVal := AuthValue{
		Authorization: "",
		ServerMode:    ServerModeSTDIO,
	}

	if len(config.Config.Username) > 0 {
		authVal.Username = config.Config.Username
	} else {
		authVal.Username = "anonymous"
	}

	if len(config.Config.Password) > 0 {
		authVal.Password = config.Config.Password
	}

	return authVal
}

func (a *AuthValue) IsSTDIO() bool {
	return a.ServerMode == "stdio"
}

func (a *AuthValue) IsHTTP() bool {
	return a.ServerMode == "http"
}

func (a *AuthValue) IsBasicAuth() bool {
	return strings.HasPrefix(a.Authorization, "Basic ")
}

func (a *AuthValue) IsBearerAuth() bool {
	return strings.HasPrefix(a.Authorization, "Bearer ")
}

func (a *AuthValue) IsAnonymous() bool {
	return a.Username == "anonymous"
}

func (a *AuthValue) getAuthToken() string {
	if a.IsBasicAuth() {
		return strings.TrimPrefix(a.Authorization, "Basic ")
	} else if a.IsBearerAuth() {
		return strings.TrimPrefix(a.Authorization, "Bearer ")
	}
	return ""
}

func (a *AuthValue) parseBasicAuth() (string, string) {
	if a.IsBasicAuth() {
		authToken := a.getAuthToken()
		if !strings.Contains(authToken, ":") {
			// possibly base64 encoded string
			decodedAuthToken, err := base64.StdEncoding.DecodeString(authToken)
			if err == nil {
				authToken = string(decodedAuthToken)
			}
		}

		username := ""
		password := ""
		authArr := strings.Split(authToken, ":")
		if len(authArr) > 0 {
			username = authArr[0]
		}

		if len(authArr) > 1 {
			password = authArr[1]
		}

		return username, password
	}

	return "", ""
}

func (a *AuthValue) parseUsernameFromJWT() string {
	if a.IsBearerAuth() {
		authToken := a.getAuthToken()
		parts := strings.Split(authToken, ".")
		if len(parts) != 3 {
			return ""
		}

		// base64url decoding
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}

		// extract preferred_username
		var claims map[string]interface{}
		if err := json.Unmarshal(payload, &claims); err != nil {
			return ""
		}
		if username, ok := claims["preferred_username"].(string); ok {
			return username
		}
	}

	return ""
}

func GetAuthValue(ctx context.Context) (AuthValue, error) {
	authVal, ok := ctx.Value(AuthKey{}).(AuthValue)
	if !ok {
		return AuthValue{}, errors.New("failed to get auth value from context")
	}
	return authVal, nil
}
