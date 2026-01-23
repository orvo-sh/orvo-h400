package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/ingestservice"
	"github.com/orvo-sh/orvo/internal/http/helper"
	"github.com/orvo-sh/orvo/internal/http/middleware/bodyparser"
)

type ingestHttpHandler struct {
	ingestService ingestservice.Service
}

type (
	ingestEvent_Request struct {
		Body struct {
			ID             string          `json:"id"`
			Timestamp      time.Time       `json:"timestamp"`
			Level          models.LogLevel `json:"level"`
			Message        string          `json:"message"`
			Service        string          `json:"service"`
			Environment    string          `json:"environment"`
			OrganizationID string          `json:"organization_id"`
			ParentID       string          `json:"parent_id"`
			Attributes     map[string]any  `json:"attributes"`
		} `in:"body=json"`
	}
)

func SetupIngestHttpHandler(r chi.Router, ingestService ingestservice.Service) {
	controller := ingestHttpHandler{
		ingestService: ingestService,
	}

	r.Route("/v1/logs", func(r chi.Router) {
		r.With(bodyparser.New[ingestEvent_Request]()).Post("/", controller.ingestLog)
	})
}

func (h *ingestHttpHandler) ingestLog(w http.ResponseWriter, r *http.Request) {
	body := bodyparser.GetBodyFromContext[ingestEvent_Request](r).Body

	_, err := h.ingestService.IngestLogEvent(r.Context(), ingestservice.IngestLogInput{
		Timestamp:      body.Timestamp,
		Level:          body.Level,
		Message:        body.Message,
		Service:        body.Service,
		Environment:    body.Environment,
		OrganizationID: body.OrganizationID,
		ParentID:       body.ParentID,
		Attributes:     body.Attributes,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}
	helper.Resp(w, nil, nil)
}
