package main

import (
	"fmt"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/http/handlers"
	"github.com/orvo-sh/orvo/pkg/util"
)

func main() {
	config := util.Must(config.Load())

	r := chi.NewRouter()

	humaConfig := huma.DefaultConfig("Orvo API", "1.0.0")
	humaConfig.Servers = []*huma.Server{
		{URL: "/api/v1"},
	}
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: config.Session.CookieName,
		},
	}

	api := humachi.New(r, humaConfig)

	handlers.NewAuthHandler(nil, handlers.NewAuthConfig{}).RegisterRoutes(api)
	handlers.NewOrganizationHandler(nil, nil).RegisterRoutes(api)
	handlers.NewApiKeyHandler(nil).RegisterRoutes(api)
	handlers.NewLogHandler(nil, nil).RegisterRoutes(api)

	spec, err := api.OpenAPI().YAML()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(spec))
}
