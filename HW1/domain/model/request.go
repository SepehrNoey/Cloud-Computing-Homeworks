package model

import "errors"

type Request struct {
	ID     int
	Email  string
	Status string
	SongID string
}

var LastRegisteredRequestID = 0
var ErrRequestSavingDBFailure = errors.New("couldn't save request in database")
var ErrRequestSavingQueueFailure = errors.New("couldn't save request id in RabbitMQ")
