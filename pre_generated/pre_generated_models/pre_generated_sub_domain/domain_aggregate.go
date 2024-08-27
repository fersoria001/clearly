package pre_generated_sub_domain

import (
	"clearly-not-a-secret-project/lazy_loading"
)

type DomainAggregate struct {
	id         string
	name       string
	loadStatus lazy_loading.LoadStatus
}

func NewDomainAggregate(id, name string) *DomainAggregate {
	return &DomainAggregate{
		id:         id,
		name:       name,
		loadStatus: lazy_loading.LOADED,
	}
}
