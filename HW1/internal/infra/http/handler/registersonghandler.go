package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/repository/requestrepo"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/repository/songrepo"
	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailAndSong struct {
	Email  string `json:"email"`
	Song   string `json:"song_base64"`
	Format string `json:"format"`
}

type RegisterSongHandler struct {
	reqRepo  requestrepo.Repository
	songRepo songrepo.Repository
	channel  *amqp.Channel
	queue    *amqp.Queue
	logFile  *os.File
}

func NewRegisterSongHandler(reqRepo requestrepo.Repository, songRepo songrepo.Repository, channel *amqp.Channel, queue *amqp.Queue, logFile *os.File) *RegisterSongHandler {
	return &RegisterSongHandler{
		reqRepo:  reqRepo,
		songRepo: songRepo,
		channel:  channel,
		queue:    queue,
		logFile:  logFile,
	}
}

func (h *RegisterSongHandler) RegisterSong(w http.ResponseWriter, r *http.Request) {
	model.LastRegisteredRequestID++

	var es EmailAndSong
	if err := json.NewDecoder(r.Body).Decode(&es); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// save request info in DB
	req := model.Request{
		ID:     model.LastRegisteredRequestID,
		Email:  es.Email,
		Status: string(model.Pending),
		SongID: model.Unknown,
	}
	if err := h.reqRepo.Create(r.Context(), req); err != nil {
		http.Error(w, model.ErrRequestSavingDBFailure.Error(), http.StatusInternalServerError)
		Log(h.logFile, WrapWithRequestID(model.ErrRequestSavingDBFailure.Error(), req.ID))
		return
	}

	// save song to object storage
	binaryData, _ := base64.StdEncoding.DecodeString(es.Song)
	if err := h.songRepo.Create(r.Context(), model.Song{
		ID:             model.Unknown,
		ReqID:          model.LastRegisteredRequestID,
		SongDataBinary: binaryData,
		SongFormat:     es.Format,
	}); err != nil {
		http.Error(w, model.ErrSongSavingFailure.Error(), http.StatusInternalServerError)
		if err = SetFailure(h.reqRepo, req.ID, model.ErrSongSavingFailure.Error()); err != nil {
			Log(h.logFile, WrapWithRequestID(err.Error(), req.ID))
		}
		Log(h.logFile, WrapWithRequestID(model.ErrSongSavingFailure.Error(), req.ID))
		return
	}

	// add to rabbitMQ
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	msgBody := fmt.Sprintf("%v", model.LastRegisteredRequestID)
	fmt.Printf("msgBody to be sent to rabbit is: %v\n", msgBody)
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
		fmt.Printf("error occurred pushing message to rabbit: %s\n", err.Error())
		http.Error(w, model.ErrRequestAddingToQueueFailure.Error(), http.StatusInternalServerError)
		if err = SetFailure(h.reqRepo, req.ID, model.ErrRequestAddingToQueueFailure.Error()); err != nil {
			Log(h.logFile, WrapWithRequestID(err.Error(), req.ID))
		}
		Log(h.logFile, WrapWithRequestID(model.ErrRequestAddingToQueueFailure.Error(), req.ID))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "song registered successfully"}`))
}
