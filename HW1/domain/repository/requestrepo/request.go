package requestrepo

import (
	"context"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks.git/domain/model"
)

type Repository interface {
	Create(ctx context.Context, req model.Request) error
	Update(ctx context.Context, id int, updatedReq model.Request) error
	Get(ctx context.Context, id int) *model.Request
}
