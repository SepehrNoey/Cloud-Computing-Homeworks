package model

import "errors"

type Request struct {
	ID              int
	Email           string
	Status          string
	SongID          string
	ErrorMessage    string
	IsMailAttempted bool
}

var LastRegisteredRequestID = 0
var ErrRequestSavingDBFailure = errors.New("couldn't save request in database")
var ErrRequestAddingToQueueFailure = errors.New("couldn't add request id to RabbitMQ")
