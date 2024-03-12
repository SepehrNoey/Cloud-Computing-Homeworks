package songrepo

import (
	"context"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/model"
)

type Repository interface {
	Create(ctx context.Context, song model.Song) error
	Get(ctx context.Context, reqID int) *model.Song
}
