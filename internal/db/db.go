package db

import (
	"fmt"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)		

type Pool struct {
	Conn *pgxpool.Pool
}

func Connect(ctx context.Context, databaseUrl string) (*Pool, error){

	// Create new Pool
	pool , err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		return  nil , err
	}

	// verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil , err
	}
	fmt.Println("Connected to Postgres Successfull")

	// return pool
	return &Pool{
		Conn : pool,
	}, nil
}


func (p *Pool) Close() {
	// Close the pool 
	if p != nil && p.Conn != nil {
		p.Conn.Close()
	}
}