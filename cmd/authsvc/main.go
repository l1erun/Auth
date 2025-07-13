package main

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/example/auth/internal/app"
)

func main() {
	dsn := "postgres://user:pass@localhost:5432/auth?sslmode=disable"
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	srv := app.New(db, rdb)

	go func() {
		if err := srv.RunGRPC(":50051"); err != nil {
			log.Println("grpc", err)
		}
	}()

	if err := srv.RunHTTP(":8080"); err != nil {
		log.Println("http", err)
	}
}
