package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/repository/requestrepo"
)

func Log(logFile *os.File, msg string) {
	log.SetOutput(logFile)
	log.Print(msg)
}

func SetFailure(reqRepo requestrepo.Repository, id int, errMsg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	reqs := reqRepo.Get(ctx, requestrepo.GetCommand{
		ID: &id,
	})

	if len(reqs) > 1 || len(reqs) == 0 {
		return errors.New(WrapWithRequestID("zero or multiple request with this id found", id))
	}
	req := reqs[0]

	err := reqRepo.Update(ctx, req.ID, model.Request{
		ID:           req.ID,
		Email:        req.Email,
		Status:       string(model.Failure),
		SongID:       req.SongID,
		ErrorMessage: errMsg,
	})
	return err

}

func WrapWithRequestID(msg string, id int) string {
	return fmt.Sprintf("%s, request id: %v\n", msg, id)
}
