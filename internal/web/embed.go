package web

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var frontendFS embed.FS

func RegisterFrontend(r *gin.Engine) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		log.Fatalf("failed to load frontend: %v", err)
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
