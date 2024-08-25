package interfaces

import "reflect"

type Registrable interface {
	Type() reflect.Type
}
