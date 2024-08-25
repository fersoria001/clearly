package lazy_loading

type LoadStatus int

const (
	GHOST = iota
	LOADING
	LOADED
)
