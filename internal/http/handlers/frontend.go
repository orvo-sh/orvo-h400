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

		modTime := time.Now()
		if info, err := index.Stat(); err == nil {
			modTime = info.ModTime()
		}
		http.ServeContent(w, r, "index.html", modTime, index)
	})
}
