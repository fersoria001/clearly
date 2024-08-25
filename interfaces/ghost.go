package interfaces

type Ghost interface {
	IsGhost() bool
	IsLoaded() bool
	MarkLoading() error
	MarkLoaded() error
}
