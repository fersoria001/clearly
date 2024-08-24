package example

import "github.com/google/uuid"

type DomainAggregate struct {
	id   string
	name string
	//entity *DomainEntity
	// valueObject *DomainValueObject
}

func NewDomainAggregate(id, name string) *DomainAggregate {
	return &DomainAggregate{
		id:   id,
		name: name,
	}
}

func (a *DomainAggregate) SetName(name string) {
	a.name = name
}

func (a DomainAggregate) Id() string {
	return a.id
}

func (a DomainAggregate) Name() string {
	return a.name
}

type DomainValueObject struct {
	id    string
	value int
}

type DomainEntity struct {
	id          uuid.UUID
	name        string
	valueObject *DomainValueObject
}
