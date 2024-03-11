package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/internal/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/internal/domain/repository/requestrepo"
	"github.com/mailgun/mailgun-go/v4"
)

type SongRecommendHandler struct {
	reqRepo     requestrepo.Repository
	logFile     *os.File
	mg          *mailgun.MailgunImpl
	senderEmail string
}

func NewSongRecommendHandler(reqRepo requestrepo.Repository, logFile *os.File, mg *mailgun.MailgunImpl, senderEmail string) *SongRecommendHandler {
	return &SongRecommendHandler{
		reqRepo:     reqRepo,
		logFile:     logFile,
		mg:          mg,
		senderEmail: senderEmail,
	}
}

func (h *SongRecommendHandler) ReadAndEmailSimilar() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	readyStr := string(model.Ready)
	notMailed := false
	reqs := h.reqRepo.Get(ctx, requestrepo.GetCommand{
		Status:          &readyStr,
		IsMailAttempted: &notMailed,
	})
	cancel()

	// no ready requests to process
	if len(reqs) == 0 {
		cancel()
		return
	}

	for _, req := range reqs {
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		recs, err := h.GetRecommendedSongs(ctx, req.ID, req.SongID)
		if err != nil {
			if err2 := SetFailure(h.reqRepo, req.ID, err.Error()); err2 != nil {
				Log(h.logFile, WrapWithRequestID(err2.Error(), req.ID))
			}
			Log(h.logFile, WrapWithRequestID(err.Error(), req.ID))
			failMsg := h.mg.NewMessage(h.senderEmail, "Song Recommendation Failure",
				fmt.Sprintf("Your request failed due to some error: %s\nRequest ID: %v\nSong ID at Spotify: %s", err.Error(), req.ID, req.SongID),
				req.Email)

			_, _, err2 := h.mg.Send(ctx, failMsg)

			if err2 != nil {
				h.reqRepo.Update(context.Background(), req.ID, model.Request{
					ID:              req.ID,
					Email:           req.Email,
					Status:          string(model.Failure),
					SongID:          req.SongID,
					ErrorMessage:    err2.Error(),
					IsMailAttempted: true,
				})
				Log(h.logFile, WrapWithRequestID(err2.Error(), req.ID))

			} else {
				h.reqRepo.Update(ctx, req.ID, model.Request{
					ID:              req.ID,
					Email:           req.Email,
					Status:          string(model.Failure),
					SongID:          req.SongID,
					ErrorMessage:    err.Error(),
					IsMailAttempted: true,
				})
				Log(h.logFile, WrapWithRequestID(err.Error(), req.ID))
			}

			cancel()
			continue
		}

		content, err := json.MarshalIndent(recs, "", "\t")
		if err != nil {
			Log(h.logFile, WrapWithRequestID("failed to marshal recommendations", req.ID))
			cancel()
			continue
		}

		succMsg := h.mg.NewMessage(h.senderEmail, "Song Recommendation Success", string(content), req.Email)
		_, _, err = h.mg.Send(ctx, succMsg)
		if err != nil {
			h.reqRepo.Update(context.Background(), req.ID, model.Request{
				ID:              req.ID,
				Email:           req.Email,
				Status:          string(model.Failure),
				SongID:          req.SongID,
				ErrorMessage:    err.Error(),
				IsMailAttempted: true,
			})
			Log(h.logFile, WrapWithRequestID(err.Error(), req.ID))
			cancel()
			continue
		}

		h.reqRepo.Update(context.Background(), req.ID, model.Request{
			ID:              req.ID,
			Email:           req.Email,
			Status:          string(model.Done),
			SongID:          req.SongID,
			IsMailAttempted: true,
		})
		cancel()
	}
}

type SpotifyRecommendAlbum struct {
	Name        string `json:"name"`
	ReleaseDate string `json:"release_date"`
}

type SpotifyRecommendArtist struct {
	Name string `json:"name"`
}

type SpotifyRecommendTrack struct {
	Album      SpotifyRecommendAlbum    `json:"album"`
	Artists    []SpotifyRecommendArtist `json:"artists"`
	Name       string                   `json:"name"`
	PreviewURL string                   `json:"preview_url"`
}

type SpotifyRecommendResponse struct {
	Tracks []SpotifyRecommendTrack `json:"tracks"`
}

func (h *SongRecommendHandler) GetRecommendedSongs(ctx context.Context, id int, songID string) (*SpotifyRecommendResponse, error) {
	url := fmt.Sprintf("https://spotify23.p.rapidapi.com/recommendations/?limit=20&seed_tracks=%s", songID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	req.Header.Add("X-RapidAPI-Key", "3acee9ab9amshc1a9e394e2bcbccp1b469fjsn92fa96f0bc42")
	req.Header.Add("X-RapidAPI-Host", "spotify23.p.rapidapi.com")

	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf(": %s, request id: %v\n", err.Error(), id)
		return nil, errors.New("error in reading body of Spotify response for song recommendation")
	}

	var recs SpotifyRecommendResponse
	if err = json.Unmarshal(body, &recs); err != nil {
		return nil, errors.New("error in unmarshalling Spotify song recommend response body")
	}

	return &recs, nil
}
