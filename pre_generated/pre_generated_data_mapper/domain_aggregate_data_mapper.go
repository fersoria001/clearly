package pre_generated_data_mapper

import (
	"clearly-not-a-secret-project/data_mapper"
	"clearly-not-a-secret-project/pre_generated/pre_generated_models/pre_generated_sub_domain"

	"clearly-not-a-secret-project/interfaces"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DomainAggregateDataMapper struct {
	data_mapper.PostgreSQLDataMapper[interfaces.DomainObject[string], string]
}

func NewDomainAggregateDataMapper(pool *pgxpool.Pool, loadedMap map[string]interfaces.DomainObject[string]) *DomainAggregateDataMapper {
	return &DomainAggregateDataMapper{
		PostgreSQLDataMapper: data_mapper.PostgreSQLDataMapper[interfaces.DomainObject[string], string]{
			Db:              pool,
			LoadedMap:       loadedMap,
			FindStatement:   `SELECT ID, NAME FROM AGGREGATE WHERE ID = $1;`,
			InsertStatement: `INSERT INTO AGGREGATE (ID, NAME) VALUES ($1, $2);`,
			UpdateStatement: `UPDATE AGGREGATE SET NAME = $2 WHERE ID = $1`,
			RemoveStatement: `DELETE FROM AGGREGATE WHERE ID = $1;`,
			DoLoad: func(resultSet pgx.Rows) (interfaces.DomainObject[string], error) {
				var (
					id   string
					name string
				)
				err := resultSet.Scan(&id, &name)
				if err != nil {
					return nil, err
				}
				return pre_generated_sub_domain.NewDomainAggregate(id, name), nil
			},
			DoInsert: func(obj interfaces.DomainObject[string], stmt *data_mapper.PreparedStatement) error {
				subject, ok := obj.(*pre_generated_sub_domain.DomainAggregate)
				if !ok {
					return fmt.Errorf("wrong type assertion")
				}
				stmt.Append(subject.Id())
				stmt.Append(subject.Name())
				return nil
			},
			DoUpdate: func(obj interfaces.DomainObject[string], stmt *data_mapper.PreparedStatement) error {
				subject, ok := obj.(*pre_generated_sub_domain.DomainAggregate)
				if !ok {
					return fmt.Errorf("wrong type assertion")
				}
				stmt.Append(subject.Id())
				stmt.Append(subject.Name())
				return nil
			},
			DomainType:  reflect.TypeOf(&pre_generated_sub_domain.DomainAggregate{}),
			LazyLoading: false,
		},
	}
}
