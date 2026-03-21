package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var frontendFS embed.FS

func RegisterFrontend(r *gin.Engine) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		slog.Error("failed to load frontend", "error", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(distFS))
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) > 1 {
			f, err := distFS.Open(path[1:])
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
