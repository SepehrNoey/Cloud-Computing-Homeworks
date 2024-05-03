package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/handler"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Config Map
	cm, err := clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "my-config-map", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	db, _ := strconv.Atoi(cm.Data["REDIS_DB"])

	rdsClient := redis.NewClient(&redis.Options{
		Addr:     cm.Data["REDIS_ADDR"],
		Password: cm.Data["REDIS_PASSWORD"],
		DB:       db,
	})

	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{cm.Data["ELASTICSEARCH_ADDR"]},
	})
	if err != nil {
		panic(err)
	}

	sh := handler.New(rdsClient, esClient)

	fmt.Println("Server starting...")
	http.HandleFunc("/", sh.SearchMovie)
	http.ListenAndServe(cm.Data["SERVER_LISTEN_ADDR"], nil)

}
