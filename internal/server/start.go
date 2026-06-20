package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/i18n"
)

func ServerAddr() string {
	return fmt.Sprintf("%s:%d", config.AppConfig.Server.Host, config.AppConfig.Server.Port)
}

func tlsFilesPresent() bool {
	cert := config.AppConfig.Server.TLSCertFile
	key := config.AppConfig.Server.TLSKeyFile
	if cert == "" || key == "" {
		return false
	}
	if _, err := os.Stat(cert); err != nil {
		return false
	}
	if _, err := os.Stat(key); err != nil {
		return false
	}
	return true
}

// Start levanta HTTPS si está configurado y los ficheros existen, en caso contrario levanta HTTP.
// Con TLS y http_redirect_port > 0, abre además HTTP en ese puerto con la misma app Fiber:
// clientes AP usan HTTP para el portal cautivo; el resto redirige a HTTPS vía middleware.
// Mantiene un shutdown gracioso con señal SIGINT/SIGTERM.
func Start(app *fiber.App) {
	addr := ServerAddr()
	listenHost := config.AppConfig.Server.Host
	if listenHost == "" {
		listenHost = "0.0.0.0"
	}

	i18n.LogTf("logs.server_starting", addr)
	i18n.LogTf("logs.server_config",
		config.AppConfig.Server.Debug,
		config.AppConfig.Server.ReadTimeout,
		config.AppConfig.Server.WriteTimeout)

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		i18n.LogTln("logs.server_stopping")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(ctx); err != nil {
			i18n.LogTf("logs.server_shutdown_error", err)
		}
		os.Exit(0)
	}()

	i18n.LogTf("logs.server_ready", addr)

	useTLS := tlsFilesPresent()
	if useTLS {
		httpPort := config.AppConfig.Server.HTTPRedirectPort
		if httpPort > 0 && httpPort != config.AppConfig.Server.Port {
			httpAddr := fmt.Sprintf("%s:%d", listenHost, httpPort)
			i18n.LogTf("logs.server_http_captive_listen", httpAddr)
			go func() {
				if err := app.Listen(httpAddr); err != nil {
					i18n.LogTf("logs.server_http_captive_error", err)
				}
			}()
		} else if httpPort > 0 {
			i18n.LogTln("logs.server_http_redirect_port_conflict")
		}
		if err := app.ListenTLS(addr, config.AppConfig.Server.TLSCertFile, config.AppConfig.Server.TLSKeyFile); err != nil {
			i18n.LogTfatal("logs.server_start_error", err)
		}
		return
	}

	if err := app.Listen(addr); err != nil {
		i18n.LogTfatal("logs.server_start_error", err)
	}
}
