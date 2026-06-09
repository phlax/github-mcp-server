package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/utils"
)

// redactToken returns the first 6 characters of token followed by "…", or "-"
// if the token is empty. It is used to produce a safe log-friendly prefix that
// never reveals the full secret value.
func redactToken(token string) string {
	if token == "" {
		return "-"
	}
	runes := []rune(token)
	if len(runes) <= 6 {
		return string(runes) + "…"
	}
	return string(runes[:6]) + "…"
}

func ExtractUserToken(logger *slog.Logger, oauthCfg *oauth.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger.Info("auth: request received",
				"method", r.Method,
				"path", r.URL.Path,
				"authorization_present", r.Header.Get("Authorization") != "",
				"mcp_session_id_present", r.Header.Get("Mcp-Session-Id") != "",
				"content_type", r.Header.Get("Content-Type"),
			)

			// Check if token info already exists in context, if it does, skip extraction.
			// In remote setup, we may have already extracted token info earlier.
			if existing, ok := ghcontext.GetTokenInfo(ctx); ok {
				logger.Info("auth: token_info already present, skipping extraction",
					"token_type", existing.TokenType,
				)
				next.ServeHTTP(w, r)
				return
			}

			tokenType, token, err := utils.ParseAuthorizationHeader(r)
			if err != nil {
				// For missing Authorization header, return 401 with WWW-Authenticate header per MCP spec
				if errors.Is(err, utils.ErrMissingAuthorizationHeader) {
					resourcePath := oauth.ResolveResourcePath(r, oauthCfg)
					resourceMetadataURL := oauth.BuildResourceMetadataURL(r, oauthCfg, resourcePath)
					logger.Warn("auth: missing Authorization header — returning 401 WWW-Authenticate",
						"resource_metadata_url", resourceMetadataURL,
					)
					sendAuthChallenge(w, r, oauthCfg)
					return
				}
				// For other auth errors (bad format, unsupported), return 400
				logger.Warn("auth: malformed Authorization header — returning 400",
					"error", err.Error(),
				)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			logger.Info("auth: extracted token",
				"token_type", tokenType,
				"token_prefix", redactToken(token),
			)

			ctx = ghcontext.WithTokenInfo(ctx, &ghcontext.TokenInfo{
				Token:     token,
				TokenType: tokenType,
			})
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// sendAuthChallenge sends a 401 Unauthorized response with WWW-Authenticate header
// containing the OAuth protected resource metadata URL as per RFC 6750 and MCP spec.
func sendAuthChallenge(w http.ResponseWriter, r *http.Request, oauthCfg *oauth.Config) {
	resourcePath := oauth.ResolveResourcePath(r, oauthCfg)
	resourceMetadataURL := oauth.BuildResourceMetadataURL(r, oauthCfg, resourcePath)
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q`, resourceMetadataURL))
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
