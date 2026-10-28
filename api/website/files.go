// Package website holds the website's templates and static files, compiled
// into the binary so the server doesn't need them on disk.
package website

import "embed"

// Files has templates/new/*.tmpl and the static/ files the pages use. Only
// xterm's built script and stylesheet are included, not its sources or maps.
//
//go:embed templates/new static/css static/img static/js
//go:embed static/node_modules/xterm/lib/xterm.js static/node_modules/xterm/css static/node_modules/xterm/LICENSE
var Files embed.FS
