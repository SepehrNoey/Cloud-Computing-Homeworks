package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/repository/requestrepo"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/repository/songrepo"
	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailAndSong struct {
	Email  string `json:"email"`
	Song   string `json:"song"`
	Format string `json:"format"`
}

type RegisterSongHandler struct {
	reqRepo  requestrepo.Repository
	songRepo songrepo.Repository
	channel  *amqp.Channel
	queue    *amqp.Queue
}

func NewRegisterSongHandler(reqRepo requestrepo.Repository, songRepo songrepo.Repository, channel *amqp.Channel, queue *amqp.Queue) *RegisterSongHandler {
	return &RegisterSongHandler{
		reqRepo:  reqRepo,
		songRepo: songRepo,
		channel:  channel,
		queue:    queue,
	}
}

func (h *RegisterSongHandler) RegisterSong(w http.ResponseWriter, r *http.Request) {
	var es EmailAndSong

	if err := json.NewDecoder(r.Body).Decode(&es); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// save song to object storage
	if err := h.songRepo.Create(r.Context(), model.Song{
		ID:             model.Unknown,
		ReqID:          model.LastRegisteredRequestID + 1,
		SongDataBase64: es.Song,
		SongFormat:     es.Format,
	}); err != nil {
		http.Error(w, model.ErrSongSavingFailure.Error(), http.StatusInternalServerError)
		return
	}

	// save request info in DB
	if err := h.reqRepo.Create(r.Context(), model.Request{
		ID:     model.LastRegisteredRequestID + 1,
		Email:  es.Email,
		Status: string(model.Pending),
		SongID: model.Unknown,
	}); err != nil {
		http.Error(w, model.ErrRequestSavingDBFailure.Error(), http.StatusInternalServerError)
		return
	}

	// add to rabbitMQ
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	msgBody := fmt.Sprintf("%v", model.LastRegisteredRequestID+1)
	err := h.channel.PublishWithContext(ctx,
		"",
		h.queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(msgBody),
		})
	if err != nil {
		http.Error(w, model.ErrRequestSavingQueueFailure.Error(), http.StatusInternalServerError)
		return
	}
	model.LastRegisteredRequestID++

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "song registered successfully"}`))
}
