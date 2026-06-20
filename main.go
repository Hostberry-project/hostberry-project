package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"hostberry/internal/app"
	"hostberry/internal/constants"
	"hostberry/internal/i18n"
	server "hostberry/internal/server"
)

//go:embed website/templates
var templatesFS embed.FS

//go:embed website/static
var staticFS embed.FS

func main() {
	showVersion := flag.Bool("version", false, "mostrar versión y salir")
	flag.Parse()
	if *showVersion {
		fmt.Println(constants.Version)
		os.Exit(0)
	}

	fiberApp, err := app.Bootstrap(templatesFS, staticFS)
	if err != nil {
		i18n.LogTfatal("logs.config_load_error", err)
	}
	server.Start(fiberApp)
}
