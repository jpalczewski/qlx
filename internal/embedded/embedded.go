package embedded

import "embed"

//go:embed templates/*.html templates/partials/*.html templates/components/*.html
var Templates embed.FS

//go:embed static/*
var Static embed.FS
