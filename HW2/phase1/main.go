package main

import (
	"fmt"
	"net/http"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/phase1/handler"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
)

func main() {
	rdsClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	esClient, err := elasticsearch.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	sh := handler.New(rdsClient, esClient)

	fmt.Println("Server starting...")
	http.HandleFunc("/", sh.SearchMovie)
	http.ListenAndServe(":2024", nil)

}