package ui

// Asset loading. The bundled font and icon bytes are compiled into the binary
// as base64 source in assets_gen.go (produced by tools/genassets, run via
// `make generate`) rather than via //go:embed — so the data flow is explicit:
// every asset is fetched by path through resource(), and the binary stays
// self-contained with no runtime file I/O.

import (
	"path"

	"fyne.io/fyne/v2"
)

// resource loads a bundled asset by its path under internal/ui (e.g.
// "fonts/IBMPlexSans-Regular.ttf") and wraps it as a named Fyne resource. The
// resource name is the base filename, matching how Fyne caches resources.
// Unknown names panic: a missing asset is a build-time mistake, not a runtime
// condition to handle.
func resource(name string) fyne.Resource {
	data, ok := embeddedAssets[name]
	if !ok {
		panic("ui: unknown bundled asset " + name)
	}
	return fyne.NewStaticResource(path.Base(name), data)
}
