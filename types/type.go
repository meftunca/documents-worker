package types

type MediaKind string

const (
	ImageKind MediaKind = "image"
	VideoKind MediaKind = "video"
	DocKind   MediaKind = "document"
)

type MediaSearch struct {
	Width       *int
	Height      *int
	Crop        *string
	Quality     *int
	ResizeScale *int
	CutVideo    *string
	Page        *int
}

type MediaConverter struct {
	Kind        MediaKind
	Search      MediaSearch
	Format      *string
	VipsEnabled bool
}
