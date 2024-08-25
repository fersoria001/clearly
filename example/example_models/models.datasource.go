package example_models

import (
	"clearly-not-a-secret-project/example/example_registry"
	"clearly-not-a-secret-project/interfaces"
	"fmt"
)

func Load[K comparable](obj interfaces.DomainObject[K]) error {
	instance, err := example_registry.Instance[K]()
	if err != nil {
		return fmt.Errorf("error in concrete data source %w", err)
	}
	err = instance.Load(obj)
	if err != nil {
		return fmt.Errorf("error in concrete data source %w", err)
	}
	return nil
}
