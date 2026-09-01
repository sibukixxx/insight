// Package httpapi wires the HTTP router. It lives under internal/http for
// directory-naming consistency with the design doc, but is declared as
// httpapi (not http) so callers can import it alongside net/http without
// needing an alias.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"insight-lab/internal/http/handler"
	appmw "insight-lab/internal/http/middleware"
	"insight-lab/internal/repository"
	"insight-lab/internal/service"
	"insight-lab/internal/web"
)

type Deps struct {
	Projects  repository.ProjectRepository
	Documents repository.DocumentRepository
	Demo      *service.DemoLoader
	Build     handler.BuildInfo
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(appmw.RestrictOrigin)

	h := handler.New(deps.Projects, deps.Documents, deps.Demo, deps.Build)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.Health)
		r.Post("/demo", h.CreateDemoProject)

		r.Route("/projects", func(r chi.Router) {
			r.Get("/", h.ListProjects)
			r.Post("/", h.CreateProject)

			r.Route("/{projectID}", func(r chi.Router) {
				r.Get("/", h.GetProject)
				r.Delete("/", h.DeleteProject)
				r.Get("/documents", h.ListDocuments)
				r.Post("/documents", h.CreateDocument)
			})
		})

		r.Get("/documents/{documentID}", h.GetDocument)
	})

	r.Handle("/*", web.Handler())
	return r
}
