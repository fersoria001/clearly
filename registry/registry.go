package registry

import (
	"clearly-not-a-secret-project/data_mapper"
	"clearly-not-a-secret-project/interfaces"
	"fmt"
	"reflect"
)

type Registries map[reflect.Type]any

type Registry[K comparable] struct {
	m map[reflect.Type]interfaces.Registrable
}

func New[K comparable]() *Registry[K] {
	return &Registry[K]{
		m: make(map[reflect.Type]interfaces.Registrable, 0),
	}
}

func (r *Registry[K]) Register(obj interfaces.Registrable) {
	if r.m == nil {
		panic("Registry is nil, it must be initialized first")
	}
	if _, ok := r.m[obj.Type()]; ok {
		return
	}
	r.m[obj.Type()] = obj
}

func (r *Registry[K]) Mapper(typeName reflect.Type) (data_mapper.DataMapper[interfaces.DomainObject[K], K], error) {
	v, ok := r.m[typeName]
	if !ok {
		return nil, fmt.Errorf("the mapper for type %v is not in the registry", typeName)
	}
	typeOfConcreteDataMapper := reflect.TypeOf(v)
	typeOfDataMapperInterface := reflect.TypeOf((*data_mapper.DataMapper[interfaces.DomainObject[K], K])(nil)).Elem()
	if !typeOfConcreteDataMapper.Implements(typeOfDataMapperInterface) {
		return nil, fmt.Errorf("registered type %s does not implement DataMapper in Mapper()", typeOfConcreteDataMapper.Name())
	}
	mapper, ok := v.(data_mapper.DataMapper[interfaces.DomainObject[K], K])
	if !ok {
		return nil, fmt.Errorf("type %s cant be casted to the data mapper interface", typeOfConcreteDataMapper.Name())
	}
	return mapper, nil
}

func (r *Registry[K]) Load(obj interfaces.DomainObject[K]) error {
	mapper, err := r.Mapper(obj.Type())
	if err != nil {
		return err
	}
	return mapper.Load(obj)
}
