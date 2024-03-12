package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Request struct {
	ID              int            `json:"id,omitempty" gorm:"primaryKey"`
	Email           string         `json:"email"`
	Status          string         `json:"status"`
	SongID          string         `json:"song_id"`
	ErrorMessage    string         `json:"error_message"`
	IsMailAttempted bool           `json:"is_mail_attempted"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at,omitempty"`
	DeletedAt       gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

var LastRegisteredRequestID = 0
var ErrRequestSavingDBFailure = errors.New("couldn't save request in database")
var ErrRequestAddingToQueueFailure = errors.New("couldn't add request id to RabbitMQ")
var ErrRequestIdDuplicate = errors.New("request id already exists")
