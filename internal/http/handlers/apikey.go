package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type ApiKeyHandler struct {
	authService authservice.Service
}

func NewApiKeyHandler(authService authservice.Service) *ApiKeyHandler {
	return &ApiKeyHandler{
		authService: authService,
	}
}

func (h *ApiKeyHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "create-api-key",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/api-keys",
		Tags:        []string{"api-keys"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.createApiKey)

	huma.Register(api, huma.Operation{
		OperationID: "list-api-keys",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/api-keys",
		Tags:        []string{"api-keys"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listApiKeys)

	huma.Register(api, huma.Operation{
		OperationID: "revoke-api-key",
		Method:      http.MethodDelete,
		Path:        "/organizations/{organization_id}/api-keys/{key_id}",
		Tags:        []string{"api-keys"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.revokeApiKey)
}

func (h *ApiKeyHandler) createApiKey(ctx context.Context, input *dto.CreateApiKeyInput) (*dto.CreateApiKeyOutput, error) {
	if apiKey, rawKey, err := h.authService.CreateApiKey(ctx, authservice.CreateApiKeyInput{
		OrganizationID: input.OrganizationID,
		Name:           input.Body.Name,
		ExpiresIn:      input.Body.ExpiresIn,
	}); err != nil {
		return nil, err
	} else {
		return &dto.CreateApiKeyOutput{
			Body: struct {
				ApiKey models.ApiKey `json:"api_key"`
				Key    string        `json:"key"`
			}{
				ApiKey: *apiKey,
				Key:    *rawKey,
			},
		}, nil
	}
}

func (h *ApiKeyHandler) listApiKeys(ctx context.Context, input *dto.ListApiKeysInput) (*dto.ListApiKeysOutput, error) {
	if keys, err := h.authService.ListApiKeys(ctx, input.OrganizationID); err != nil {
		return nil, err
	} else {
		return &dto.ListApiKeysOutput{
			Body: struct {
				ApiKeys []models.ApiKey `json:"api_keys"`
			}{
				ApiKeys: keys,
			},
		}, nil
	}
}

func (h *ApiKeyHandler) revokeApiKey(ctx context.Context, input *dto.RevokeApiKeyInput) (*struct{}, error) {
	if err := h.authService.RevokeApiKey(ctx, authservice.RevokeApiKeyInput{
		ID:             input.KeyID,
		OrganizationID: input.OrganizationID,
	}); err != nil {
		return nil, err
	}
	return nil, nil
}
