package example_tests

import (
	"clearly-not-a-secret-project/example/example_conn"
	"clearly-not-a-secret-project/example/example_data_mapper"
	"clearly-not-a-secret-project/example/example_models"
	"clearly-not-a-secret-project/example/example_registry"
	"clearly-not-a-secret-project/interfaces"
	"testing"
)

func TestRegistry_Load(t *testing.T) {
	pool, err := example_conn.CreatePool()
	if err != nil {
		t.Fatal(err)
	}
	loadedMap := make(map[string]interfaces.DomainObject[string], 0)
	newMapper := example_data_mapper.NewDomainAggregateDataMapper(pool, loadedMap)
	reg, err := example_registry.Instance[string]()
	if err != nil {
		t.Fatal(err)
	}
	reg.Register(newMapper)
	obj := example_models.CreateDomainAggregateGhost("1")
	err = reg.Load(obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRegistry_Mapper(t *testing.T) {
	pool, err := example_conn.CreatePool()
	if err != nil {
		t.Fatal(err)
	}
	loadedMap := make(map[string]interfaces.DomainObject[string], 0)
	newMapper := example_data_mapper.NewDomainAggregateDataMapper(pool, loadedMap)
	reg, err := example_registry.Instance[string]()
	if err != nil {
		t.Fatal(err)
	}
	reg.Register(newMapper)
	obj := example_models.NewDomainAggregate("1", "name")
	_, err = reg.Mapper(obj.Type())
	if err != nil {
		t.Fatal(err)
	}
}
