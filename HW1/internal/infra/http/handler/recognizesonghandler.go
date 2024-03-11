package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/internal/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/internal/domain/repository/requestrepo"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/internal/domain/repository/songrepo"
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
		Log(h.logFile, err.Error())
		return
	}

	for msg := range msgs {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

		reqIDStr := string(msg.Body)
		id, err := strconv.Atoi(reqIDStr)
		if err != nil {
			Log(h.logFile, err.Error())
			cancel()
			return
		}

		songData, err := h.ReadSongFromObjectStorage(ctx, id)
		if err != nil {
			if err2 := SetFailure(h.reqRepo, id, err.Error()); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID(err.Error(), id))
			cancel()
			continue
		}

		song, err := h.DecodeBase64(songData.SongDataBase64, id)
		if err != nil {
			if err2 := SetFailure(h.reqRepo, id, err.Error()); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID(err.Error(), id))
			cancel()
			continue
		}

		res, err := h.RequestToShazam(song, songData.SongFormat, id)
		if err != nil {
			if err2 := SetFailure(h.reqRepo, id, err.Error()); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID(err.Error(), id))
			cancel()
			continue
		}

		spotResp, err := h.SearchInSpotify(res.Track.Title, id)
		if err != nil {
			if err2 := SetFailure(h.reqRepo, id, err.Error()); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID(err.Error(), id))
			cancel()
			continue
		}

		items := spotResp.Tracks.Items
		if len(items) == 0 {
			if err2 := SetFailure(h.reqRepo, id, "song not found in Spotify"); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID("song not found in Spotify", id))
			cancel()
			continue
		}

		reqsInDB := h.reqRepo.Get(ctx, requestrepo.GetCommand{ID: &id})
		if len(reqsInDB) > 1 {
			if err2 := SetFailure(h.reqRepo, id, "multiple requests with same id exist in database"); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID("multiple requests with same id exist in database", id))
			cancel()
			continue
		}
		if len(reqsInDB) == 0 {
			if err2 := SetFailure(h.reqRepo, id, "song not found in database"); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID("song not found in database", id))
			cancel()
			continue
		}

		reqInDB := reqsInDB[0]
		if err := h.reqRepo.Update(ctx, id, model.Request{
			ID:     id,
			Email:  reqInDB.Email,
			Status: string(model.Ready),
			SongID: items[0].Data.ID,
		}); err != nil {
			if err2 := SetFailure(h.reqRepo, id, "adding song id in database failed"); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), id))
			}
			Log(h.logFile, WrapWithRequestID("adding song id in database failed", id))
			cancel()
			continue
		}

		cancel()
	}
}

func (h *RecognizeSongHandler) ReadSongFromObjectStorage(ctx context.Context, id int) (*model.Song, error) {
	songData := h.songRepo.Get(ctx, id)
	if songData == nil {
		return nil, errors.New("associated song to request is nil")
	}

	return songData, nil
}

func (h *RecognizeSongHandler) DecodeBase64(encdStr string, id int) (*[]byte, error) {
	song, err := base64.StdEncoding.DecodeString(encdStr)
	if err != nil {
		return nil, model.ErrSongDataDecodeFailure
	}

	return &song, nil
}

type ShazamTrack struct {
	Title string `json:"title"`
}

type ShazamResponse struct {
	Track ShazamTrack `json:"track"`
}

func (h *RecognizeSongHandler) RequestToShazam(song *[]byte, songFormat string, id int) (*ShazamResponse, error) {
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	part, err := writer.CreateFormFile("upload_file", fmt.Sprintf("song.%s", songFormat))

	if err != nil {
		return nil, errors.New("error creatig form file")
	}

	_, err = part.Write(*song)
	if err != nil {
		return nil, errors.New("error writing song data")
	}

	err = writer.Close()
	if err != nil {
		return nil, errors.New("error closing writer")
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
		return nil, errors.New("error sending request or getting response for Shazam")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.New("error in reading response body for Shazam")
	}
	res.Body.Close()

	var jsonResp ShazamResponse
	if err = json.Unmarshal(body, &jsonResp); err != nil {
		return nil, errors.New("error in unmarshalling Shazam response body")
	}

	return &jsonResp, nil
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

func (h *RecognizeSongHandler) SearchInSpotify(title string, id int) (*SpotifySongSearchResponse, error) {
	encodedTitle := url.QueryEscape(title)
	url := fmt.Sprintf("https://spotify23.p.rapidapi.com/search/?q=%s&type=tracks&offset=0&limit=10&numberOfTopResults=5", encodedTitle)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("X-RapidAPI-Key", "3acee9ab9amshc1a9e394e2bcbccp1b469fjsn92fa96f0bc42")
	req.Header.Add("X-RapidAPI-Host", "spotify23.p.rapidapi.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("error in sending request to Spotify or getting response")
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.New("error in reading body of Spotify response for search by song title")
	}

	var jsonResp SpotifySongSearchResponse
	if err = json.Unmarshal(body, &jsonResp); err != nil {
		return nil, errors.New("error in unmarshalling Spotify song search response body")
	}

	return &jsonResp, nil
}
