package httpapi

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// maxStaticFileBytes soft-caps SPA asset buffering (#242).
// Production assets are well under 1 MiB; this is defense-in-depth.
const maxStaticFileBytes = 8 << 20 // 8 MiB

func registerStaticRoutes(r interface{ NotFound(http.HandlerFunc) }, staticFS fs.FS) {
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if isAPIRoute(req.URL.Path) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}

		name, ok := cleanStaticPath(req.URL.Path)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid static path")
			return
		}
		if name == "." || name == "" {
			name = "index.html"
		}

		file, err := staticFS.Open(name)
		if err != nil {
			file, err = staticFS.Open("index.html")
			if err != nil {
				writeError(w, http.StatusNotFound, "static index not found")
				return
			}
			name = "index.html"
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil || info.IsDir() {
			writeError(w, http.StatusNotFound, "static file not found")
			return
		}
		if info.Size() > maxStaticFileBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "static file too large")
			return
		}
		if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		// Prefer streaming when the FS file is seekable (os.DirFS); otherwise bound ReadAll.
		if seeker, ok := file.(io.ReadSeeker); ok {
			http.ServeContent(w, req, name, info.ModTime(), seeker)
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, maxStaticFileBytes+1))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read static file")
			return
		}
		if len(data) > maxStaticFileBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "static file too large")
			return
		}
		http.ServeContent(w, req, name, info.ModTime(), bytes.NewReader(data))
	})
}

func isAPIRoute(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/api/") || requestPath == "/api" || requestPath == "/mcp"
}

func cleanStaticPath(requestPath string) (string, bool) {
	unescaped, err := url.PathUnescape(requestPath)
	if err != nil {
		return "", false
	}
	if strings.Contains(unescaped, string(rune(0))) {
		return "", false
	}
	parts := strings.Split(unescaped, "/")
	for _, part := range parts {
		if part == ".." {
			return "", false
		}
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+unescaped), "/")
	return cleaned, true
}
