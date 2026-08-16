package web

import (
	"log/slog"
	"net/http"
	"os"
)

const DefaultStaticDir = "web/static"

func staticHandler(dir string, logger *slog.Logger) http.Handler {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		logger.Warn("static directory not found, no assets will be served until it exists", "dir", dir)
	}
	return http.FileServer(http.Dir(dir))
}
