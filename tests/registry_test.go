package example_tests

import (
	"clearly-not-a-secret-project/interfaces"
	"clearly-not-a-secret-project/pre_generated/pre_generated_conn"
	"clearly-not-a-secret-project/pre_generated/pre_generated_data_mapper"
	"clearly-not-a-secret-project/pre_generated/pre_generated_models/pre_generated_sub_domain"
	"clearly-not-a-secret-project/pre_generated/pre_generated_registry"
	"testing"
)

func TestRegistry_Mapper(t *testing.T) {
	pool, err := pre_generated_conn.CreatePool()
	if err != nil {
		t.Fatal(err)
	}
	loadedMap := make(map[string]interfaces.DomainObject[string], 0)
	newMapper := pre_generated_data_mapper.NewDomainAggregateDataMapper(pool, loadedMap)
	reg, err := pre_generated_registry.Instance[string]()
	if err != nil {
		t.Fatal(err)
	}
	reg.Register(newMapper)
	obj := pre_generated_sub_domain.NewDomainAggregate("1", "name")
	_, err = reg.Mapper(obj.Type())
	if err != nil {
		t.Fatal(err)
	}
}
