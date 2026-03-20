package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var embeddedDist embed.FS

func newStaticHandler() (http.Handler, error) {
	distFS, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil, err
	}
	return spaHandler(distFS), nil
}

// spaHandler 提供静态文件服务，对不存在的无扩展名路径回退到 index.html（SPA fallback）。
func spaHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API 和健康检查路径不走静态文件
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// 尝试打开请求的文件
		urlPath := strings.TrimPrefix(r.URL.Path, "/")
		if urlPath == "" {
			urlPath = "index.html"
		}

		f, err := root.Open(urlPath)
		if err != nil {
			// 文件不存在：仅对无扩展名的 HTML 请求做 SPA fallback
			ext := path.Ext(urlPath)
			if ext == "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
