package interfaces

type Comparable interface {
	Equals(obj Comparable) bool
}
