package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/repository/requestrepo"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/repository/songrepo"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RecognizeSongHandler struct {
	reqRepo  requestrepo.Repository
	songRepo songrepo.Repository
	channel  *amqp.Channel
	queue    *amqp.Queue
	logFile  *os.File
}

func NewRecognizeSongHandler(reqRepo requestrepo.Repository, songRepo songrepo.Repository, channel *amqp.Channel, queue *amqp.Queue, logFile *os.File) *RecognizeSongHandler {
	return &RecognizeSongHandler{
		reqRepo:  reqRepo,
		songRepo: songRepo,
		channel:  channel,
		queue:    queue,
		logFile:  logFile,
	}
}

func (h *RecognizeSongHandler) ReadAndRecognize() {
	log.SetOutput(h.logFile)

	msgs, err := h.channel.Consume(
		h.queue.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("failed to register a consumer on channel: %s\n", err.Error())
		return
	}

	for msg := range msgs {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

		reqIDStr := string(msg.Body)
		id, err := strconv.Atoi(reqIDStr)
		if err != nil {
			log.Printf("failed to convert message body to integer as request ID: %s\n", err.Error())
			cancel()
			return
		}

		songData := h.ReadSongFromObjectStorage(ctx, id)
		if songData == nil {
			cancel()
			continue
		}

		song := h.DecodeBase64(songData.SongDataBase64, id)
		if song == nil {
			cancel()
			continue
		}

		res := h.RequestToShazam(song, songData.SongFormat, id)
		if res == nil {
			cancel()
			continue
		}

		spotResp := h.SearchInSpotify(res.Track.Title, id)
		if spotResp == nil {
			cancel()
			continue
		}

		items := spotResp.Tracks.Items
		if len(items) == 0 {
			log.Printf("song not found in Spotify, request id: %v\n", id)
			cancel()
			continue
		}

		reqInDB := h.reqRepo.Get(ctx, id)
		if reqInDB == nil {
			log.Printf("song not found in database, request id:%v\n", id)
			cancel()
			continue
		}

		if err := h.reqRepo.Update(ctx, id, model.Request{
			ID:     id,
			Email:  reqInDB.Email,
			Status: string(model.Ready),
			SongID: items[0].Data.ID,
		}); err != nil {
			log.Printf("adding song id in database failed, request id:%v\n", id)
			cancel()
			continue
		}

		cancel()
	}
}

func (h *RecognizeSongHandler) ReadSongFromObjectStorage(ctx context.Context, id int) *model.Song {
	songData := h.songRepo.Get(ctx, id)
	if songData == nil {
		log.Printf("associated song to request with id: %v is nil\n", id)
		return nil
	}

	return songData
}

func (h *RecognizeSongHandler) DecodeBase64(encdStr string, id int) *[]byte {
	song, err := base64.StdEncoding.DecodeString(encdStr)
	if err != nil {
		log.Printf("%s, decoding error: %s, request id: %v\n", model.ErrSongDataDecodeFailure.Error(), err.Error(), id)
		return nil
	}

	return &song
}

type ShazamTrack struct {
	Title string `json:"title"`
}

type ShazamResponse struct {
	Track ShazamTrack `json:"track"`
}

func (h *RecognizeSongHandler) RequestToShazam(song *[]byte, songFormat string, id int) *ShazamResponse {
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	part, err := writer.CreateFormFile("upload_file", fmt.Sprintf("song.%s", songFormat))

	if err != nil {
		log.Printf("error creatig form file: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	_, err = part.Write(*song)
	if err != nil {
		log.Printf("error writing song data: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	err = writer.Close()
	if err != nil {
		log.Printf("error closing writer: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	// creating an http request
	url := "https://shazam-api-free.p.rapidapi.com/shazam/recognize/"
	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("X-RapidAPI-Key", "3acee9ab9amshc1a9e394e2bcbccp1b469fjsn92fa96f0bc42")
	req.Header.Add("X-RapidAPI-Host", "shazam-api-free.p.rapidapi.com")

	// send the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("error sending request or getting response: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error in reading response body: %s, request id: %v\n", err.Error(), id)
		return nil
	}
	res.Body.Close()

	var jsonResp ShazamResponse
	if err = json.Unmarshal(body, &jsonResp); err != nil {
		log.Printf("error in unmarshalling Shazam response body: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	return &jsonResp
}

type SpotifyItemData struct {
	URI  string `json:"uri"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SpotifyItem struct {
	Data SpotifyItemData `json:"data"`
}

type SpotifyTracks struct {
	Items []SpotifyItem `json:"items"`
}

type SpotifySongSearchResponse struct {
	Tracks SpotifyTracks `json:"tracks"`
}

func (h *RecognizeSongHandler) SearchInSpotify(title string, id int) *SpotifySongSearchResponse {
	encodedTitle := url.QueryEscape(title)
	url := fmt.Sprintf("https://spotify23.p.rapidapi.com/search/?q=%s&type=tracks&offset=0&limit=10&numberOfTopResults=5", encodedTitle)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("X-RapidAPI-Key", "3acee9ab9amshc1a9e394e2bcbccp1b469fjsn92fa96f0bc42")
	req.Header.Add("X-RapidAPI-Host", "spotify23.p.rapidapi.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error in sending request to Spotify or getting response: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error in reading body of Spotify response for search by song title: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	var jsonResp SpotifySongSearchResponse
	if err = json.Unmarshal(body, &jsonResp); err != nil {
		log.Printf("error in unmarshalling Spotify response body: %s, request id: %v\n", err.Error(), id)
		return nil
	}

	return &jsonResp
}
