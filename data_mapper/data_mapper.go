package data_mapper

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatementSource interface {
	Sql() string
	Parameters() []interface{}
}

type Recognizable[K any] interface {
	Id() K
}

type Comparable interface {
	Equals(obj Comparable) bool
}

type DataMapper[T Recognizable[K], K comparable] interface {
	Insert(ctx context.Context, obj T) (K, error)
	Update(ctx context.Context, obj T) error
	Remove(ctx context.Context, id K) error
	Find(ctx context.Context, id K) (T, error)
	FindMany(ctx context.Context, source StatementSource) ([]T, error)
	getId(rows pgx.Rows) (K, error)
	load(resultSet pgx.Rows) (T, error)
	loadAll(resultSet pgx.Rows) ([]T, error)
	//registries.Registrable
}

type PreparedStatement struct {
	conn  *pgxpool.Pool
	query string
	args  []interface{}
}

func (q *PreparedStatement) Append(arg interface{}) {
	q.args = append(q.args, arg)
}

func (q *PreparedStatement) Execute(ctx context.Context) (int64, error) {
	cmd, err := q.conn.Exec(ctx, q.query, q.args...)
	if err != nil {
		return cmd.RowsAffected(), err
	}
	return cmd.RowsAffected(), nil
}

func (q *PreparedStatement) ExecuteQuery(ctx context.Context) (pgx.Rows, error) {
	rows, err := q.conn.Query(ctx, q.query, q.args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

type DomainObject[K comparable] interface {
	Recognizable[K]
	//Comparable
	//Markable
}

type Markable interface {
	MarkNew() error
	MarkClean() error
	MarkDirty() error
	MarkRemoved() error
}

type PostgreSQLDataMapper[T Recognizable[K], K comparable] struct {
	Db              *pgxpool.Pool
	LoadedMap       map[K]T
	FindStatement   string
	InsertStatement string
	UpdateStatement string
	RemoveStatement string
	DoLoad          func(resultSet pgx.Rows) (T, error)
	DoInsert        func(obj T, stmt *PreparedStatement) error
	DoUpdate        func(obj T, stmt *PreparedStatement) error
	DomainType      reflect.Type
}

func (d PostgreSQLDataMapper[T, K]) Type() reflect.Type {
	return d.DomainType
}

func (d PostgreSQLDataMapper[T, K]) Insert(ctx context.Context, obj T) (K, error) {
	var nilK K
	stmt := &PreparedStatement{
		conn:  d.Db,
		query: d.InsertStatement,
		args:  make([]interface{}, 0),
	}
	err := d.DoInsert(obj, stmt)
	if err != nil {
		return nilK, err
	}
	_, err = stmt.Execute(ctx)
	if err != nil {
		return nilK, err
	}
	id := obj.Id()
	d.LoadedMap[id] = obj
	return id, nil
}

func (d PostgreSQLDataMapper[T, K]) Update(ctx context.Context, obj T) error {
	stmt := &PreparedStatement{
		conn:  d.Db,
		query: d.UpdateStatement,
		args:  make([]interface{}, 0),
	}
	err := d.DoUpdate(obj, stmt)
	if err != nil {
		return err
	}
	_, err = stmt.Execute(ctx)
	if err != nil {
		return err
	}
	id := obj.Id()
	d.LoadedMap[id] = obj
	return nil
}

func (d PostgreSQLDataMapper[T, K]) Remove(ctx context.Context, id K) error {
	stmt := &PreparedStatement{
		conn:  d.Db,
		query: d.RemoveStatement,
		args:  make([]interface{}, 0),
	}
	stmt.Append(id)
	_, err := stmt.Execute(ctx)
	if err != nil {
		return err
	}
	delete(d.LoadedMap, id)
	return nil
}

func (d PostgreSQLDataMapper[T, K]) Find(ctx context.Context, id K) (T, error) {
	var nilT T
	if obj, ok := d.LoadedMap[id]; ok {
		return obj, nil
	}
	stmt := &PreparedStatement{
		conn:  d.Db,
		query: d.FindStatement,
		args:  make([]interface{}, 0),
	}
	stmt.Append(id)
	rows, err := stmt.ExecuteQuery(ctx)
	if err != nil {
		return nilT, fmt.Errorf("error at execute query %w", err)
	}
	rows.Next()
	return d.load(rows)
}

func (d PostgreSQLDataMapper[T, K]) FindMany(ctx context.Context, source StatementSource) ([]T, error) {
	stmt := &PreparedStatement{
		conn:  d.Db,
		query: d.RemoveStatement,
		args:  make([]interface{}, 0),
	}
	rows, err := stmt.ExecuteQuery(ctx)
	if err != nil {
		return nil, err
	}
	result, err := d.loadAll(rows)
	if err != nil {
		return nil, err
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (d PostgreSQLDataMapper[T, K]) getId(rows pgx.Rows) (K, error) {
	var (
		nilK  K
		index int = 0
	)
	fieldDescriptions := len(rows.FieldDescriptions())
	if index > fieldDescriptions {
		return nilK, fmt.Errorf("index out of range")
	}
	values := make([]interface{}, fieldDescriptions)
	for i := range fieldDescriptions {
		var v interface{}
		values[i] = &v
	}
	err := rows.Scan(values...)
	if err != nil {
		return nilK, err
	}
	ptr, ok := values[index].(*interface{})
	if !ok {
		return nilK, fmt.Errorf("getId wrong interface assertion %v is %v could not cast to",
			values[index],
			reflect.TypeOf(values[index]))
	}
	v := *ptr
	toId, ok := v.(K)
	if !ok {
		return nilK, fmt.Errorf("getId wrong interface assertion %v is %v could not cast to, the value does not implement comparable interface",
			toId,
			reflect.TypeOf(v))
	}
	return toId, nil
}

func (d PostgreSQLDataMapper[T, K]) load(resultSet pgx.Rows) (T, error) {
	var nilT T
	id, err := d.getId(resultSet)
	if err != nil {
		return nilT, fmt.Errorf("error at load getId %w", err)
	}
	if obj, ok := d.LoadedMap[id]; ok {
		return obj, nil
	}
	result, err := d.DoLoad(resultSet)
	if err != nil {
		return nilT, fmt.Errorf("error at doLoad %w", err)
	}
	d.LoadedMap[id] = result
	return result, nil
}

func (d PostgreSQLDataMapper[T, K]) loadAll(resultSet pgx.Rows) ([]T, error) {
	result := make([]T, 0)
	for resultSet.Next() {
		obj, err := d.load(resultSet)
		if err != nil {
			return nil, err
		}
		result = append(result, obj)
	}
	return result, nil
}
