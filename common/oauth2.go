package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/cockroachdb/errors"

	log "github.com/sirupsen/logrus"
)

type OAuth2 struct {
	// the /mcp endpoint that MCP server is hosted on
	McpURL string
	// URL of the "issuer" for .well-known/openid-configuration
	AuthorizationURL string
	// URL to .well-known/openid-configuration
	OIDCDiscoveryURL           string
	tokenIntrospectionEndpoint string // discovered from OIDCDiscoveryURL
	userinfoEndpoint           string // discovered from OIDCDiscoveryURL

	// Client ID and secret to validate access token
	ClientID     string
	ClientSecret string
}

type OIDCDiscoveryResponse struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	TokenIntrospectionEndpoint                 string   `json:"token_introspection_endpoint"`
	UserinfoEndpoint                           string   `json:"userinfo_endpoint"`
	EndSessionEndpoint                         string   `json:"end_session_endpoint"`
	JwksUri                                    string   `json:"jwks_uri"`
	CheckSessionIframe                         string   `json:"check_session_iframe"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	SubjectTypesSupported                      []string `json:"subject_types_supported"`
	IdTokenSigningAlgValuesSupported           []string `json:"id_token_signing_alg_values_supported"`
	IdTokenEncryptionAlgValuesSupported        []string `json:"id_token_encryption_alg_values_supported"`
	IdTokenEncryptionEncValuesSupported        []string `json:"id_token_encryption_enc_values_supported"`
	UserinfoSigningAlgValuesSupported          []string `json:"userinfo_signing_alg_values_supported"`
	RequestObjectSigningAlgValuesSupported     []string `json:"request_object_signing_alg_values_supported"`
	ResponseModesSupported                     []string `json:"response_modes_supported"`
	RegistrationEndpoint                       string   `json:"registration_endpoint"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
	ClaimsSupported                            []string `json:"claims_supported"`
	ClaimTypesSupported                        []string `json:"claim_types_supported"`
	ClaimsParameterSupported                   bool     `json:"claims_parameter_supported"`
	ScopesSupported                            []string `json:"scopes_supported"`
	RequestParameterSupported                  bool     `json:"request_parameter_supported"`
	RequestUriParameterSupported               bool     `json:"request_uri_parameter_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	TlsClientCertificateBoundAccessTokens      bool     `json:"tls_client_certificate_bound_access_tokens"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint"`
}

func NewOAuth2(McpURL string, OIDCDiscoveryURL string, clientID, clientSecret string) (*OAuth2, error) {
	resp, err := http.Get(OIDCDiscoveryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody OIDCDiscoveryResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return nil, err
	}
	if respBody.IntrospectionEndpoint == "" {
		return nil, fmt.Errorf("the OIDC discovery document does not contain the token introspection endpoint")
	}
	if respBody.UserinfoEndpoint == "" {
		return nil, fmt.Errorf("the OIDC discovery document does not contain the userinfo endpoint")
	}

	return &OAuth2{
		McpURL:                     McpURL,
		AuthorizationURL:           respBody.Issuer,
		OIDCDiscoveryURL:           OIDCDiscoveryURL,
		tokenIntrospectionEndpoint: respBody.IntrospectionEndpoint,
		userinfoEndpoint:           respBody.UserinfoEndpoint,
		ClientID:                   clientID,
		ClientSecret:               clientSecret,
	}, nil
}

// RequireOAuth is a middleware that requires OAuth2 access token in the header, or else return 401 with WWW-Authenticate header.
func (o *OAuth2) RequireOAuth(next http.Handler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		logger := log.WithFields(log.Fields{
			"uri":    request.RequestURI,
			"method": request.Method,
		})
		logger.Debug("Request received, checking oauth")
		authHeader := request.Header.Get("Authorization")
		if authHeader == "" {
			logger.Error("Authorization header is missing")
			o.unauthorizedResponse(writer, request)
			return
		}
		token, isBearer := strings.CutPrefix(authHeader, "Bearer ")
		if !isBearer {
			logger.Error("Authorization header is not bearer token")
			o.unauthorizedResponse(writer, request)
			return
		}
		if token == "" {
			logger.Error("token is empty")
			o.unauthorizedResponse(writer, request)
			return
		}
		logger = logger.WithField("token", token)
		logger.Debug("bearer token in auth header")
		valid, err := o.oauthIntrospectToken(o.tokenIntrospectionEndpoint, o.ClientID, o.ClientSecret, token)
		if err != nil {
			logger.WithError(err).Error("Failed to introspect token")
			o.unauthorizedResponse(writer, request)
			return
		}
		if !valid {
			logger.Error("invalid token")
			o.unauthorizedResponse(writer, request)
			return
		}
		userinfo, err := o.oauthGetUserinfo(o.userinfoEndpoint, token)
		if err != nil {
			log.WithError(err).Error("Failed to get userinfo for token")
			o.unauthorizedResponse(writer, request)
			return
		}

		logger.WithFields(log.Fields{"username": userinfo.PreferredUsername, "sub": userinfo.Sub}).Infoln("Request received, user is authenticated")

		// propagate the username to auth module for irods access
		// userinfo.PreferredUsername is expected to be the iRODS username
		request.Header.Set("X-Forwarded-User", userinfo.PreferredUsername)

		next.ServeHTTP(writer, request)
	}
}

func (o *OAuth2) oauthGetUserinfo(userinfoEndpoint string, token string) (UserInfo, error) {
	resp, err := http.PostForm(userinfoEndpoint, url.Values{
		"access_token": {token},
	})
	if err != nil {
		return UserInfo{}, err
	}
	var userInfo UserInfo
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		return UserInfo{}, err
	}
	return userInfo, nil
}

type UserInfo struct {
	Sub               string `json:"sub"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Email             string `json:"email"`
}

func (o *OAuth2) oauthIntrospectToken(inspectEndpoint string, oauthClientID, oauthClientSecret, accessToken string) (bool, error) {
	data := url.Values{
		"token": {accessToken},
	}
	request, err := http.NewRequest(http.MethodPost, inspectEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"endpoint": inspectEndpoint,
		}).Error("Failed to create request")
		return false, err
	}
	request.SetBasicAuth(oauthClientID, oauthClientSecret)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"endpoint": inspectEndpoint,
		}).Error("Failed to do request")
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		all, err2 := io.ReadAll(resp.Body)
		if err2 != nil {
			log.WithError(err2).Error("Failed to read response body")
			return false, err2
		}
		log.WithFields(log.Fields{
			"code": resp.StatusCode,
			"body": string(all),
		}).Error("Failed to inspect token")
		return false, fmt.Errorf("failed to inspect token: %s", resp.Status)
	}
	var claims map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&claims)
	if err != nil {
		log.WithError(err).Error("Failed to unmarshal claims from response body")
		return false, err
	}
	log.WithFields(log.Fields{
		"claims": claims,
	}).Info("Successfully inspect token")

	active, ok := claims["active"]
	if !ok {
		msg := "the token introspection response did not contain the active flag"
		log.Error(msg)
		return false, errors.Errorf(msg)
	}
	switch isActive := active.(type) {
	case bool:
		if !isActive {
			msg := "invalid or expired access token"
			log.WithField("claims", claims).Error(msg)
			return false, errors.Errorf(msg)
		}
	default:
		msg := "invalid value for active flag"
		log.WithField("claims", claims).Error(msg)
		return false, errors.Errorf(msg)
	}
	log.WithFields(log.Fields{
		"token":  accessToken,
		"active": active,
		"claims": claims,
	}).Info("Successfully inspect token, token is active")

	return true, nil
}

type ResourceMetadata struct {
	ResourceName           string   `json:"resource_name,omitempty"`
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	JwksURI                string   `json:"jwks_uri,omitempty"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ResourcePolicyURI      string   `json:"resource_policy_uri,omitempty"`
	ResourceTOSURI         []string `json:"resource_tos_uri,omitempty"`
}

type AuthorizationMetadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	JwksURI                                    string   `json:"jwks_uri,omitempty"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	ResponseModesSupported                     []string `json:"response_modes_supported"`
	RegistrationEndpoint                       string   `json:"registration_endpoint"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
	ScopesSupported                            []string `json:"scopes_supported"`
	RequestParameterSupported                  bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported               bool     `json:"request_uri_parameter_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	TLSClientCertificateBoundAccessTokens      bool     `json:"tls_client_certificate_bound_access_tokens"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint"`
	TokenIntrospectionEndpoint                 string   `json:"token_introspection_endpoint"`
	RevocationEndpoint                         string   `json:"revocation_endpoint,omitempty"`
}

func (o *OAuth2) setResponseHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

// for /.well-known/oauth-protected-resource
// and /.well-known/oauth-protected-resource/mcp
func (o *OAuth2) HandleResourceMetadataURI(w http.ResponseWriter, r *http.Request) {
	o.setResponseHeader(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metadata = ResourceMetadata{
		ResourceName: "CyVerse Data Store MCP server",
		Resource:     o.McpURL,
		AuthorizationServers: []string{
			o.AuthorizationURL,
		},
		ScopesSupported: []string{
			"openid",
			"mcp:api",
			"mcp:read",
			"mcp:write",
		},
		BearerMethodsSupported: []string{
			"header",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(metadata)
	if err != nil {
		log.Error(err.Error())
	}
}

// for /.well-known/oauth-authorization-server
// and /.well-known/oauth-authorization-server/mcp
func (o *OAuth2) HandleAuthServerMetadataURI(w http.ResponseWriter, r *http.Request) {
	o.setResponseHeader(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the OIDC Discovery URL to construct the OAuth2 authorization server metadata URL
	discoveryURL, err := url.Parse(o.OIDCDiscoveryURL)
	if err != nil {
		http.Error(w, "Invalid OIDC discovery URL", http.StatusInternalServerError)
		return
	}

	// OAuth2 authorization server metadata is typically at /.well-known/oauth-authorization-server
	authServerURL := *discoveryURL
	authServerURL.Path = "/.well-known/oauth-authorization-server"

	// Connect to the authorization server metadata endpoint and proxy the response
	resp, err := http.Get(authServerURL.String())
	if err != nil {
		http.Error(w, "Failed to get authorization server metadata", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read authorization server metadata", http.StatusInternalServerError)
		return
	}

	authMetadata := AuthorizationMetadata{}
	err = json.Unmarshal(bodyBytes, &authMetadata)
	if err != nil {
		http.Error(w, "Failed to parse authorization server metadata", http.StatusInternalServerError)
		return
	}

	// Write the response back to the client
	jsonBytes, err := json.Marshal(authMetadata)
	if err != nil {
		http.Error(w, "Failed to marshal authorization server metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(jsonBytes)
}

// for /.well-known/openid-configuration
// and /.well-known/openid-configuration/mcp
func (o *OAuth2) HandleOIDCDiscoveryURI(w http.ResponseWriter, r *http.Request) {
	o.setResponseHeader(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Connect to the authorization server metadata endpoint and proxy the response
	resp, err := http.Get(o.OIDCDiscoveryURL)
	if err != nil {
		http.Error(w, "Failed to get authorization server metadata", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to read authorization server metadata", http.StatusInternalServerError)
		return
	}
}

func (o *OAuth2) unauthorizedResponse(w http.ResponseWriter, r *http.Request) {
	protectedResourceURL, err := url.Parse(o.McpURL)
	if err != nil {
		return
	}
	protectedResourceURL.Path = path.Join(".well-known/oauth-protected-resource", protectedResourceURL.Path)
	wwwAuthHeader := fmt.Sprintf(
		`Bearer resource_metadata="%s"`,
		protectedResourceURL.String(),
	)

	w.Header().Set("WWW-Authenticate", wwwAuthHeader)
	w.WriteHeader(http.StatusUnauthorized)
}
