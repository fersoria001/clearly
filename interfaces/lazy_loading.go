package interfaces

type LazyLoading[T Recognizable[K], K comparable] interface {
	Load(obj T) error
}
