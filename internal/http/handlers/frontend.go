package handlers

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/orvo-sh/orvo/frontend"
	"github.com/orvo-sh/orvo/pkg/util"
)

func FrontendHandler(r chi.Router) {
	static := http.FileSystem(http.FS(util.Must(fs.Sub(frontend.Efs, "build"))))

	r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		f, err := static.Open(path)
		if err == nil {
			f.Close()
			http.FileServer(static).ServeHTTP(w, r)
			return
		}

		index := util.Must(static.Open("200.html"))
		defer index.Close()
		http.ServeContent(w, r, "index.html", func() time.Time {
			if info, err := f.Stat(); err == nil {
				return info.ModTime()
			}
			return time.Now()
		}(), index)
	})
}
