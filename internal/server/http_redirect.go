package server

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"hostberry/internal/config"
	"hostberry/internal/i18n"
)

// httpsRedirectURL construye la URL https:// para redirigir una petición HTTP entrante.
func httpsRedirectURL(r *http.Request, httpsPort int) string {
	path := r.URL.RequestURI()
	host := r.Host
	hostOnly, _, err := net.SplitHostPort(host)
	if err != nil {
		hostOnly = host
	}
	if httpsPort == 443 {
		return fmt.Sprintf("https://%s%s", hostOnly, path)
	}
	return fmt.Sprintf("https://%s:%d%s", hostOnly, httpsPort, path)
}

// startHTTPRedirectServer abre un listener HTTP mínimo que responde 308 a HTTPS.
func startHTTPRedirectServer() {
	httpPort := config.AppConfig.Server.HTTPRedirectPort
	httpsPort := config.AppConfig.Server.Port
	host := config.AppConfig.Server.Host
	if host == "" {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", host, httpPort)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpsRedirectURL(r, httpsPort)
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	i18n.LogTf("logs.server_http_redirect_listen", addr, httpsPort)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			i18n.LogTf("logs.server_http_redirect_error", err)
		}
	}()
}
