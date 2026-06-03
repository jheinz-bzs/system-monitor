package ui

// Bundled IBM Plex font faces (SIL Open Font License 1.1). Per the design
// system (design-system-02-typography-icons.html), IBM Plex Sans is used for UI
// prose and IBM Plex Mono for all data, labels, and values. The faces are
// embedded into the binary so the app is self-contained and does not depend on
// the fonts being installed on the host.
//
// Only the weights the design uses are bundled:
//   - Sans Regular / SemiBold (600, page titles) + their italics
//   - Mono Regular (400, table/meta/status) and Medium (500, metric values,
//     column labels)

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed fonts/IBMPlexSans-Regular.ttf
var ibmPlexSansRegular []byte

//go:embed fonts/IBMPlexSans-SemiBold.ttf
var ibmPlexSansSemiBold []byte

//go:embed fonts/IBMPlexSans-Italic.ttf
var ibmPlexSansItalic []byte

//go:embed fonts/IBMPlexSans-SemiBoldItalic.ttf
var ibmPlexSansSemiBoldItalic []byte

//go:embed fonts/IBMPlexMono-Regular.ttf
var ibmPlexMonoRegular []byte

//go:embed fonts/IBMPlexMono-Medium.ttf
var ibmPlexMonoMedium []byte

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

// font holds the font resources wrapping the embedded faces above.
var font = fontSet{
	SansRegular:        fyne.NewStaticResource("IBMPlexSans-Regular.ttf", ibmPlexSansRegular),
	SansSemiBold:       fyne.NewStaticResource("IBMPlexSans-SemiBold.ttf", ibmPlexSansSemiBold),
	SansItalic:         fyne.NewStaticResource("IBMPlexSans-Italic.ttf", ibmPlexSansItalic),
	SansSemiBoldItalic: fyne.NewStaticResource("IBMPlexSans-SemiBoldItalic.ttf", ibmPlexSansSemiBoldItalic),
	MonoRegular:        fyne.NewStaticResource("IBMPlexMono-Regular.ttf", ibmPlexMonoRegular),
	MonoMedium:         fyne.NewStaticResource("IBMPlexMono-Medium.ttf", ibmPlexMonoMedium),
}
