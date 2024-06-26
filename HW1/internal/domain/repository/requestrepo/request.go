package requestrepo

import (
	"context"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/model"
)

type GetCommand struct {
	ID              *int
	Status          *string
	IsMailAttempted *bool
}

type Repository interface {
	Create(ctx context.Context, req model.Request) error
	Update(ctx context.Context, updatedReq model.Request) error
	Get(ctx context.Context, cmd GetCommand) []model.Request
}
