package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/infra/http/handler"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/infra/repository/request/requestsql"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/infra/repository/song/songobjectstorage"
	"github.com/mailgun/mailgun-go/v4"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=cloud-hw1-postgres-db-cloud-hw1-db.a.aivencloud.com user=avnadmin password=AVNS_CE9NnL5EDfUuWk7eVPf dbname=defaultdb port=19535 sslmode=require TimeZone=Asia/Tehran"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("error connecting to database: %v\n", err)
		return
	}

	if err = db.AutoMigrate(&model.Request{}); err != nil {
		fmt.Printf("failed to automigrate: %v\n", err)
	}

	reqRepo := requestsql.New(db)
	songRepo := songobjectstorage.New("59ecfc7a-743d-4789-a3df-d940180b41f1", "6abacaf26b15d7ce7fc7b0d60e9773c3c80718bfe75db999efe9cc817024b326",
		"s3.ir-thr-at1.arvanstorage.ir", "cloud-course-hw1-s3") // arvan

	rabbitURL := "amqps://uiyldvnz:2_YIv-ZgFl-JEvhyr8rGfsVeIEB42G21@mustang.rmq.cloudamqp.com/uiyldvnz"
	rabbConn, _ := amqp.Dial(rabbitURL)
	defer rabbConn.Close()

	pubChan, err := rabbConn.Channel()
	if err != nil {
		panic(err)
	}
	defer pubChan.Close()

	pubQ, err := pubChan.QueueDeclare(
		"song-recommender",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	subChan, err := rabbConn.Channel()
	if err != nil {
		panic(err)
	}
	defer subChan.Close()

	subQ, err := subChan.QueueDeclare(
		"song-recommender",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	srv1LogFile, _ := os.Create("service1-log.txt")
	srv2LogFile, _ := os.Create("service2-log.txt")
	srv3LogFile, _ := os.Create("service3-log.txt")
	defer srv1LogFile.Close()
	defer srv2LogFile.Close()
	defer srv3LogFile.Close()

	mg := mailgun.NewMailgun("sandbox212f2fe530e64c24b08b83610b01470f.mailgun.org", "af93ba39e573b4519d82bd479552f74e-2c441066-6905f6f9")

	regHandler := handler.NewRegisterSongHandler(reqRepo, songRepo, pubChan, &pubQ, srv1LogFile)
	rcgHandler := handler.NewRecognizeSongHandler(reqRepo, songRepo, subChan, &subQ, srv2LogFile)
	rcmHandler := handler.NewSongRecommendHandler(reqRepo, srv3LogFile, mg, "sepehr.nk.81@gmail.com")

	http.HandleFunc("/", regHandler.RegisterSong)
	go rcgHandler.ReadAndRecognize()
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			rcmHandler.ReadAndEmailSimilar()
		}
	}()

	fmt.Println("Starting to listen...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error: ", err)
	}
}
