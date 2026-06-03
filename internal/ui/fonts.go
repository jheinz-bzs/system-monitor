package ui

// Bundled IBM Plex font faces (SIL Open Font License 1.1). Per the design
// system (design-system-02-typography-icons.html), IBM Plex Sans is used for UI
// prose and IBM Plex Mono for all data, labels, and values. The faces live in
// fonts/ and are compiled into the binary by tools/genassets (see assets.go),
// so the app is self-contained and does not depend on the fonts being installed
// on the host.
//
// Only the weights the design uses are bundled:
//   - Sans Regular / SemiBold (600, page titles) + their italics
//   - Mono Regular (400, table/meta/status) and Medium (500, metric values,
//     column labels)

import "fyne.io/fyne/v2"

// fontSet namespaces the bundled font faces as resources, so call sites read
// font.SansRegular and the origin of each face is obvious across the package.
type fontSet struct {
	SansRegular        fyne.Resource
	SansSemiBold       fyne.Resource
	SansItalic         fyne.Resource
	SansSemiBoldItalic fyne.Resource
	MonoRegular        fyne.Resource
	MonoMedium         fyne.Resource
}

// font holds the font resources, loaded from the bundled asset bytes.
var font = fontSet{
	SansRegular:        resource("fonts/IBMPlexSans-Regular.ttf"),
	SansSemiBold:       resource("fonts/IBMPlexSans-SemiBold.ttf"),
	SansItalic:         resource("fonts/IBMPlexSans-Italic.ttf"),
	SansSemiBoldItalic: resource("fonts/IBMPlexSans-SemiBoldItalic.ttf"),
	MonoRegular:        resource("fonts/IBMPlexMono-Regular.ttf"),
	MonoMedium:         resource("fonts/IBMPlexMono-Medium.ttf"),
}
