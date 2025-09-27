package main

import "embed"

//go:embed all:web
var StaticFiles embed.FS
