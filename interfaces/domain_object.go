package interfaces

type DomainObject[K comparable] interface {
	Recognizable[K]
	Registrable
	Ghost
	//Markable
}
