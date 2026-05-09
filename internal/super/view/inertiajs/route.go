package inertiajs

import (
	"embed"
	"fmt"
	fub "io/fs"
	"log"
	constants "myapp/internal/super/constants"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func ViewRoter(router *http.ServeMux, embeddedFiles embed.FS) {

	fs := http.FileServer(http.Dir("view/public"))
	router.Handle("/", fs)

	// In development, proxy requests to Vite dev server
	if os.Getenv(constants.App_env) == constants.Dev {
		fmt.Println("Working in dev environment!")
		viteURL, err := url.Parse("http://localhost:5173")
		if err != nil {
			log.Fatal("Could not parse Vite dev server url:", err)
		}

		proxy := httputil.NewSingleHostReverseProxy(viteURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/src/assets")
			req.URL.Path = "/src/assets" + req.URL.Path
		}

		// Proxy /src/assets/*
		router.Handle("GET /src/assets/", http.StripPrefix("/src/assets", proxy))
	}

	content, _err := fub.Sub(embeddedFiles, "public/assets")
	if _err != nil {
		log.Fatal(_err)
	}
	router.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(content))))

}
