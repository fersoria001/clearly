package pre_generated_conn

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreatePool() (*pgxpool.Pool, error) {
	ctx := context.Background()
	connString := "postgres://postgres:sfdkwtf@localhost:5432/test?pool_max_conns=100&search_path=public&connect_timeout=5"
	conn, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
