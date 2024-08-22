package data_mapper

import (
	"clearly-not-a-secret-project/example"
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createPool() (*pgxpool.Pool, error) {
	ctx := context.Background()
	connString := "postgres://postgres:sfdkwtf@localhost:5432/test?pool_max_conns=100&search_path=public&connect_timeout=5"
	conn, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

type DataMapperImpl struct {
	PostgreSQLDataMapper[DomainObject[string], string]
}

func NewDataMapper() (DataMapperImpl, error) {
	var Nil DataMapperImpl
	pool, err := createPool()
	if err != nil {
		return Nil, err
	}
	loadedMap := map[string]DomainObject[string]{}
	return DataMapperImpl{
		PostgreSQLDataMapper: PostgreSQLDataMapper[DomainObject[string], string]{
			Db:              pool,
			LoadedMap:       loadedMap,
			FindStatement:   `SELECT ID, NAME FROM AGGREGATE WHERE ID = $1;`,
			InsertStatement: `INSERT INTO AGGREGATE (ID, NAME) VALUES ($1, $2);`,
			UpdateStatement: `UPDATE AGGREGATE SET NAME = $2 WHERE ID = $1`,
			RemoveStatement: `DELETE FROM AGGREGATE WHERE ID = $1;`,
			DoLoad: func(resultSet pgx.Rows) (DomainObject[string], error) {
				var (
					id   string
					name string
				)
				err := resultSet.Scan(&id, &name)
				if err != nil {
					return nil, err
				}
				return example.NewDomainAggregate(id, name), nil
			},
			DoInsert: func(obj DomainObject[string], stmt *PreparedStatement) error {
				subject, ok := obj.(*example.DomainAggregate)
				if !ok {
					return fmt.Errorf("wrong type assertion")
				}
				stmt.Append(subject.Id())
				stmt.Append(subject.Name())
				return nil
			},
			DoUpdate: func(obj DomainObject[string], stmt *PreparedStatement) error {
				subject, ok := obj.(*example.DomainAggregate)
				if !ok {
					return fmt.Errorf("wrong type assertion")
				}
				stmt.Append(subject.Id())
				stmt.Append(subject.Name())
				return nil
			},
			/*DomainType: reflect.TypeOf(&example.DomainAggregate{}), for uow*/
		},
	}, nil
}

var testData = map[string]struct {
	Id   string
	Name string
}{
	"valid": {
		Id:   "stringIdValue",
		Name: "nameValue",
	},
}

func TestDataMapper(t *testing.T) {
	ctx := context.Background()

	dataMapper, err := NewDataMapper()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Insert", func(t *testing.T) {
		for _, v := range testData {
			aggregate := example.NewDomainAggregate(v.Id, v.Name)
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
		for _, v := range testData {
			dbAggregate, err := dataMapper.Find(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
			if dbAggregate == nil {
				t.Fatal("the returned object is nil")
			}
			aggregate, ok := dbAggregate.(*example.DomainAggregate)
			if !ok {
				t.Fatalf("wrong type assertion %v is %v", aggregate, reflect.TypeOf(dbAggregate))
			}
			if aggregate.Id() != v.Id {
				t.Fatalf("expected id %s, got %s", v.Id, aggregate.Id())
			}
			if aggregate.Name() != v.Name {
				t.Fatalf("expected name %s, got %s", v.Name, aggregate.Name())
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		for _, v := range testData {
			dbAggregate, err := dataMapper.Find(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
			aggregate, ok := dbAggregate.(*example.DomainAggregate)
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
			aggregate, ok = dbAggregate.(*example.DomainAggregate)
			if !ok {
				t.Fatalf("wrong type assertion %v is %v", aggregate, reflect.TypeOf(dbAggregate))
			}
			if aggregate.Name() != newName {
				t.Fatalf("expected %s got %s", newName, aggregate.Name())
			}
		}
	})

	t.Run("Remove", func(t *testing.T) {
		for _, v := range testData {
			err := dataMapper.Remove(ctx, v.Id)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}
