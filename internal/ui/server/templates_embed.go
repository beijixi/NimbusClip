package server

import "embed"

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed assets/*
var assetFiles embed.FS
