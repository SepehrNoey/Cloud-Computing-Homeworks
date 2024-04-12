package main

import (
	"fmt"
	"net/http"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/handler"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
)

func main() {
	rdsClient := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})

	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://elasticsearch:9200"},
	})
	if err != nil {
		panic(err)
	}

	sh := handler.New(rdsClient, esClient)

	fmt.Println("Server starting...")
	http.HandleFunc("/", sh.SearchMovie)
	http.ListenAndServe(":2024", nil)

}
