package interfaces

type Recognizable[K any] interface {
	Id() K
}
