package pre_generated_sub_domain

import (
	"clearly-not-a-secret-project/interfaces"
	"clearly-not-a-secret-project/lazy_loading"
	"clearly-not-a-secret-project/pre_generated/pre_generated_models"
	"fmt"
	"reflect"
)

func CreateDomainAggregateGhost(id string) interfaces.DomainObject[string] {
	return &DomainAggregate{
		id:         id,
		loadStatus: lazy_loading.GHOST,
	}
}

func (a *DomainAggregate) load() {
	if a.IsGhost() {
		err := pre_generated_models.Load(a)
		if err != nil {
			panic(fmt.Errorf("error at domain load %v", err))
		}
	}
}

func (a *DomainAggregate) Type() reflect.Type {
	return reflect.TypeOf(a)
}

func (a *DomainAggregate) SetName(name string) {
	a.name = name
}

func (a *DomainAggregate) Id() string {
	return a.id
}

func (a *DomainAggregate) Name() string {
	a.load()
	return a.name
}

func (a DomainAggregate) IsGhost() bool {
	return a.loadStatus == lazy_loading.GHOST
}

func (a DomainAggregate) IsLoaded() bool {
	return a.loadStatus == lazy_loading.LOADED
}

func (a *DomainAggregate) MarkLoading() error {
	if !a.IsGhost() {
		return fmt.Errorf("assertion error: to change the status to loading it has to be in status ghost")
	}
	a.loadStatus = lazy_loading.LOADING
	return nil
}

func (a *DomainAggregate) MarkLoaded() error {
	if a.loadStatus != lazy_loading.LOADING {
		return fmt.Errorf("assertion error: to change the status to loaded it has to be in status loading")
	}
	a.loadStatus = lazy_loading.LOADED
	return nil
}
