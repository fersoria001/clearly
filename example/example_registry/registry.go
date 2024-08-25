package example_registry

import (
	"clearly-not-a-secret-project/registry"
	"fmt"
	"reflect"
	"sync"
)

var stringIdRegistry *registry.Registry[string]
var stringIdRegistryOnce sync.Once

func stringInstance() *registry.Registry[string] {
	stringIdRegistryOnce.Do(func() {
		stringIdRegistry = registry.New[string]()
	})
	return stringIdRegistry
}

var registries = registry.Registries{
	reflect.TypeOf(""): stringInstance(),
}

func Instance[K comparable]() (*registry.Registry[K], error) {
	var zero [0]K
	t := reflect.TypeOf(zero).Elem()
	r, ok := registries[t]
	if !ok {
		return nil, fmt.Errorf("the registry instance for identifiers of type %s is not registered", t.Name())
	}
	instance, ok := r.(*registry.Registry[K])
	if !ok {
		return nil, fmt.Errorf("could not cast the recovered registry to the given type")
	}
	return instance, nil
}
