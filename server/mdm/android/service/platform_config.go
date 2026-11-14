package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/contexts/ctxerr"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mdm/android/service/androidmgmt"
)

// NewAMAPIClientWithConfig exposes direct/provider routing without enabling global dev mode.
func NewAMAPIClientWithConfig(ctx context.Context, logger *slog.Logger, licenseKey string, cfg config.MDMConfig) (androidmgmt.Client, error) {
	if cfg.AndroidGoogleServiceCredentials != "" && cfg.AndroidProxyURL != "" {
		return nil, errors.New("direct Google credentials and Android proxy URL are mutually exclusive")
	}
	if cfg.AndroidGoogleServiceCredentials != "" {
		data, err := os.ReadFile(cfg.AndroidGoogleServiceCredentials)
		if err != nil {
			return nil, ctxerr.Wrap(ctx, err, "read Android Google service credentials")
		}
		var credentials struct {
			Type      string `json:"type"`
			ProjectID string `json:"project_id"`
		}
		if json.Unmarshal(data, &credentials) != nil || credentials.Type != "service_account" || credentials.ProjectID == "" {
			return nil, errors.New("Android Google credentials must be service account JSON with a project_id")
		}
		client := androidmgmt.NewGoogleClient(ctx, logger, func(key string) string {
			if key == "FLEET_DEV_ANDROID_GOOGLE_SERVICE_CREDENTIALS" {
				return string(data)
			}
			return ""
		})
		if client == nil {
			return nil, errors.New("initialize direct Android Google client: verify service account credentials")
		}
		return androidmgmt.NewRetryClient(client, logger), nil
	}
	if cfg.AndroidProxyURL != "" {
		u, err := url.Parse(cfg.AndroidProxyURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("Android proxy URL must be an absolute HTTPS URL without user information, query or fragment")
		}
		endpoint := strings.TrimRight(cfg.AndroidProxyURL, "/") + "/"
		client := androidmgmt.NewProxyClient(ctx, logger, licenseKey, func(key string) string {
			if key == "FLEET_DEV_ANDROID_PROXY_ENDPOINT" {
				return endpoint
			}
			return ""
		})
		if client == nil {
			return nil, errors.New("initialize Android proxy client")
		}
		return androidmgmt.NewRetryClient(client, logger), nil
	}
	client := NewAMAPIClient(ctx, logger, licenseKey)
	if client == nil {
		return nil, errors.New("initialize Android management client")
	}
	return client, nil
}

// Each reconciliation run owns its client: proxy authentication is mutable and
// must not be shared with foreground requests or another background job.
func newAuthenticatedAMAPIClient(ctx context.Context, ds fleet.Datastore, logger *slog.Logger, licenseKey string, cfg config.MDMConfig) (androidmgmt.Client, error) {
	client, err := NewAMAPIClientWithConfig(ctx, logger, licenseKey, cfg)
	if err != nil {
		return nil, ctxerr.Wrap(ctx, err, "initialize Android reconciliation client")
	}
	secret, err := getClientAuthenticationSecret(ctx, ds)
	if err != nil {
		return nil, err
	}
	if err := client.SetAuthenticationSecret(secret); err != nil {
		return nil, ctxerr.Wrap(ctx, err, "set Android client authentication secret")
	}
	// A successful empty device list from a different provider must never
	// unenroll local hosts or settle commands from the original enterprise.
	verifier := &Service{ds: ds, logger: logger, androidAPIClient: client}
	if err := verifier.VerifyExistingEnterpriseIfAny(ctx); err != nil {
		return nil, ctxerr.Wrap(ctx, err, "verify Android reconciliation provider")
	}
	return client, nil
}
