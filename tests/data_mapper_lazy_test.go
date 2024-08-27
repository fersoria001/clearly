package example_tests

import (
	"clearly-not-a-secret-project/interfaces"
	"clearly-not-a-secret-project/pre_generated/pre_generated_conn"
	"clearly-not-a-secret-project/pre_generated/pre_generated_data_mapper"
	"clearly-not-a-secret-project/pre_generated/pre_generated_models/pre_generated_sub_domain"
	"clearly-not-a-secret-project/pre_generated/pre_generated_registry"
	"context"

	"reflect"
	"testing"
)

var domainAggregateTestLazyData = map[string]struct {
	Id   string
	Name string
}{
	"valid": {
		Id:   "stringIdValue",
		Name: "nameValue",
	},
	"valid1": {
		Id:   "stringIdValue1",
		Name: "nameValue",
	},
}

func TestDomainAggregateDataMapperLazy(t *testing.T) {
	ctx := context.Background()
	pool, err := pre_generated_conn.CreatePool()
	if err != nil {
		t.Fatal(err)
	}
	loadedMap := map[string]interfaces.DomainObject[string]{}
	newMapper := pre_generated_data_mapper.NewDomainAggregateDataMapper(pool, loadedMap)
	reg, err := pre_generated_registry.Instance[string]()
	if err != nil {
		t.Fatal(err)
	}
	reg.Register(newMapper)
	dataMapper, err := reg.Mapper(reflect.TypeOf(&pre_generated_sub_domain.DomainAggregate{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Insert", func(t *testing.T) {
		for _, v := range domainAggregateTestLazyData {
			aggregate := pre_generated_sub_domain.NewDomainAggregate(v.Id, v.Name)
			id, err := dataMapper.Insert(ctx, aggregate)
			if err != nil {
				t.Fatal(err)
			}
			if id != aggregate.Id() {
				t.Fatalf("expected id %s got %s", aggregate.Id(), id)
			}
		}
	})

	t.Run("Find", func(t *testing.T) {
		for _, v := range domainAggregateTestLazyData {
			dbAggregate, err := dataMapper.Find(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
			if dbAggregate == nil {
				t.Fatal("the returned object is nil\n")
			}
			aggregate, ok := dbAggregate.(*pre_generated_sub_domain.DomainAggregate)
			if !ok {
				t.Fatalf("wrong type assertion %v is %v\n", aggregate, reflect.TypeOf(dbAggregate))
			}
			if aggregate.Id() != v.Id {
				t.Fatalf("expected id %s, got %s\n", v.Id, aggregate.Id())
			}
			if aggregate.Name() != v.Name {
				t.Fatalf("expected name %s, got %s\n", v.Name, aggregate.Name())
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		for _, v := range domainAggregateTestLazyData {
			dbAggregate, err := dataMapper.Find(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
			aggregate, ok := dbAggregate.(*pre_generated_sub_domain.DomainAggregate)
			if !ok {
				t.Fatalf("wrong type assertion %v is %v", aggregate, reflect.TypeOf(dbAggregate))
			}
			newName := "newRandomName"
			aggregate.SetName(newName)
			err = dataMapper.Update(ctx, aggregate)
			if err != nil {
				t.Fatal(err)
			}
			dbAggregate, err = dataMapper.Find(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
			aggregate, ok = dbAggregate.(*pre_generated_sub_domain.DomainAggregate)
			if !ok {
				t.Fatalf("wrong type assertion %v is %v", aggregate, reflect.TypeOf(dbAggregate))
			}
			if aggregate.Name() != newName {
				t.Fatalf("expected %s got %s", newName, aggregate.Name())
			}
		}
	})

	t.Run("Remove", func(t *testing.T) {
		for _, v := range domainAggregateTestLazyData {
			err := dataMapper.Remove(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Cleanup(func() {
		for _, v := range domainAggregateTestLazyData {
			err := dataMapper.Remove(ctx, v.Id)
			if err != nil {
				continue
			}
		}
	})
}
