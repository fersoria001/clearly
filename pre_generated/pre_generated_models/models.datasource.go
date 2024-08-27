package pre_generated_models

import (
	"clearly-not-a-secret-project/interfaces"
	"clearly-not-a-secret-project/pre_generated/pre_generated_registry"
	"fmt"
)

func Load[K comparable](obj interfaces.DomainObject[K]) error {
	instance, err := pre_generated_registry.Instance[K]()
	if err != nil {
		return fmt.Errorf("error in concrete data source %w", err)
	}
	err = instance.Load(obj)
	if err != nil {
		return fmt.Errorf("error in concrete data source %w", err)
	}
	return nil
}
