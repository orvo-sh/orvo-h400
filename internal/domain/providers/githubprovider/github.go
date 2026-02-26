package githubprovider

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppID         int64
	AppSlug       string
	PrivateKeyPEM string
	WebhookSecret string
	APIBaseURL    string
	AppBaseURL    string
	HTTPClient    *http.Client
}

type Provider interface {
	Enabled() bool
	BuildInstallationURL(state string) string
	GetInstallation(ctx context.Context, installationID int64) (*Installation, error)
	CreateInstallationToken(ctx context.Context, installationID int64) (string, time.Time, error)
	ListInstallationRepositories(ctx context.Context, installationID int64) ([]Repository, error)
	CreatePullRequest(ctx context.Context, installationID int64, input CreatePullRequestInput) (*PullRequest, error)
	ValidateWebhookSignature(payload []byte, signatureHeader string) bool
}

type Installation struct {
	ID      int64
	Account struct {
		ID    int64
		Login string
		Type  string
	}
}

type Repository struct {
	ID            int64
	Name          string
	FullName      string
	DefaultBranch string
	Private       bool
	Archived      bool
	CloneURL      string
	HTMLURL       string
}

type CreatePullRequestInput struct {
	Owner string
	Repo  string
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}

type PullRequest struct {
	Number  int64
	URL     string
	HTMLURL string
}

type client struct {
	cfg        Config
	httpClient *http.Client
	privateKey *rsa.PrivateKey
	enabled    bool
}

func New(cfg Config) (Provider, error) {
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = "https://api.github.com"
	}
	if cfg.AppBaseURL == "" {
		cfg.AppBaseURL = "https://github.com"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	c := &client{
		cfg:        cfg,
		httpClient: cfg.HTTPClient,
	}

	if cfg.AppID == 0 || strings.TrimSpace(cfg.PrivateKeyPEM) == "" {
		return c, nil
	}

	privateKey, err := parsePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	c.privateKey = privateKey
	c.enabled = true
	return c, nil
}

func (c *client) Enabled() bool {
	return c.enabled
}

func (c *client) BuildInstallationURL(state string) string {
	base := strings.TrimRight(c.cfg.AppBaseURL, "/")
	slug := strings.TrimSpace(c.cfg.AppSlug)
	if slug == "" {
		return ""
	}

	values := url.Values{}
	if strings.TrimSpace(state) != "" {
		values.Set("state", state)
	}

	return fmt.Sprintf("%s/apps/%s/installations/new?%s", base, slug, values.Encode())
}

func (c *client) GetInstallation(ctx context.Context, installationID int64) (*Installation, error) {
	if !c.enabled {
		return nil, fmt.Errorf("github provider is not configured")
	}

	req, err := c.newAuthedAppRequest(ctx, http.MethodGet, fmt.Sprintf("/app/installations/%d", installationID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github installation request failed: %s: %s", resp.Status, string(body))
	}

	var payload struct {
		ID      int64 `json:"id"`
		Account struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github installation response: %w", err)
	}

	installation := &Installation{
		ID: payload.ID,
	}
	installation.Account.ID = payload.Account.ID
	installation.Account.Login = payload.Account.Login
	installation.Account.Type = payload.Account.Type
	return installation, nil
}

func (c *client) CreateInstallationToken(ctx context.Context, installationID int64) (string, time.Time, error) {
	if !c.enabled {
		return "", time.Time{}, fmt.Errorf("github provider is not configured")
	}

	req, err := c.newAuthedAppRequest(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%d/access_tokens", installationID), strings.NewReader("{}"))
	if err != nil {
		return "", time.Time{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("request github installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", time.Time{}, fmt.Errorf("github token request failed: %s: %s", resp.Status, string(body))
	}

	var payload struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", time.Time{}, fmt.Errorf("decode github token response: %w", err)
	}
	if payload.Token == "" {
		return "", time.Time{}, fmt.Errorf("github token response did not contain a token")
	}

	expiresAt := time.Now().UTC().Add(55 * time.Minute)
	if payload.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, payload.ExpiresAt); err == nil {
			expiresAt = t
		}
	}

	return payload.Token, expiresAt, nil
}

func (c *client) ListInstallationRepositories(ctx context.Context, installationID int64) ([]Repository, error) {
	token, _, err := c.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	repositories := make([]Repository, 0)
	page := 1
	for {
		path := fmt.Sprintf("/installation/repositories?per_page=100&page=%d", page)
		req, err := c.newAuthedInstallationRequest(ctx, token, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request installation repositories: %w", err)
		}

		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, fmt.Errorf("github repositories request failed: %s: %s", resp.Status, string(body))
		}

		var payload struct {
			Repositories []struct {
				ID            int64  `json:"id"`
				Name          string `json:"name"`
				FullName      string `json:"full_name"`
				DefaultBranch string `json:"default_branch"`
				Private       bool   `json:"private"`
				Archived      bool   `json:"archived"`
				CloneURL      string `json:"clone_url"`
				HTMLURL       string `json:"html_url"`
			} `json:"repositories"`
			TotalCount int `json:"total_count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode repositories response: %w", err)
		}
		resp.Body.Close()

		for _, repo := range payload.Repositories {
			repositories = append(repositories, Repository{
				ID:            repo.ID,
				Name:          repo.Name,
				FullName:      repo.FullName,
				DefaultBranch: repo.DefaultBranch,
				Private:       repo.Private,
				Archived:      repo.Archived,
				CloneURL:      repo.CloneURL,
				HTMLURL:       repo.HTMLURL,
			})
		}

		if len(payload.Repositories) < 100 {
			break
		}
		page++
	}

	return repositories, nil
}

func (c *client) CreatePullRequest(ctx context.Context, installationID int64, input CreatePullRequestInput) (*PullRequest, error) {
	token, _, err := c.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"title": input.Title,
		"head":  input.Head,
		"base":  input.Base,
		"body":  input.Body,
		"draft": input.Draft,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode pull request payload: %w", err)
	}

	path := fmt.Sprintf("/repos/%s/%s/pulls", input.Owner, input.Repo)
	req, err := c.newAuthedInstallationRequest(ctx, token, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request create pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github create pull request failed: %s: %s", resp.Status, string(respBody))
	}

	var pr struct {
		Number  int64  `json:"number"`
		URL     string `json:"url"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode pull request response: %w", err)
	}

	return &PullRequest{
		Number:  pr.Number,
		URL:     pr.URL,
		HTMLURL: pr.HTMLURL,
	}, nil
}

func (c *client) ValidateWebhookSignature(payload []byte, signatureHeader string) bool {
	if strings.TrimSpace(c.cfg.WebhookSecret) == "" {
		return true
	}

	header := strings.TrimSpace(signatureHeader)
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}

	provided := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(c.cfg.WebhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(strings.ToLower(provided)), []byte(expected))
}

func (c *client) newAuthedAppRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	jwtToken, err := c.createAppJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.APIBaseURL, "/")+path, body)
	if err != nil {
		return nil, fmt.Errorf("build github request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *client) newAuthedInstallationRequest(ctx context.Context, installationToken string, method string, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.APIBaseURL, "/")+path, body)
	if err != nil {
		return nil, fmt.Errorf("build github installation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *client) createAppJWT() (string, error) {
	if c.privateKey == nil {
		return "", fmt.Errorf("github private key is not configured")
	}

	now := time.Now().UTC().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadMap := map[string]any{
		"iat": now - 60,
		"exp": now + 9*60,
		"iss": strconv.FormatInt(c.cfg.AppID, 10),
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return "", fmt.Errorf("encode github jwt payload: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	unsigned := header + "." + payload
	hash := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign github jwt: %w", err)
	}

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, `\n`) {
		raw = strings.ReplaceAll(raw, `\n`, "\n")
	}

	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("failed to parse github private key PEM block")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse github private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github private key is not RSA")
	}

	return key, nil
}
