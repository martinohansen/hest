package main

import (
	"embed"
	"io/fs"
)

// Embedded assets for templates and static files.
//
//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var (
	staticContent fs.FS
)

func init() {
	var err error
	staticContent, err = fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
}
