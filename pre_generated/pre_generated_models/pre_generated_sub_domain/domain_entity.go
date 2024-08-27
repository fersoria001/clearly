package pre_generated_sub_domain

import "clearly-not-a-secret-project/lazy_loading"

type DomainEntity struct {
	id         string
	name       string
	loadStatus lazy_loading.LoadStatus
}

func NewDomainEntity(id, name string) *DomainEntity {
	return &DomainEntity{
		id:         id,
		name:       name,
		loadStatus: lazy_loading.LOADED,
	}
}
