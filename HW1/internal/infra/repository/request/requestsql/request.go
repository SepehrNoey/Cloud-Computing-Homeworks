package requestsql

import (
	"context"
	"errors"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/repository/requestrepo"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Create(ctx context.Context, req model.Request) error {
	result := r.db.WithContext(ctx).Create(&req)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return model.ErrRequestIdDuplicate
		}

		return result.Error
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, updatedReq model.Request) error {
	result := r.db.WithContext(ctx).Save(&updatedReq)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (r *Repository) Get(ctx context.Context, cmd requestrepo.GetCommand) []model.Request {
	var results []model.Request
	var conditions []string
	var req model.Request

	if cmd.ID != nil {
		conditions = append(conditions, "ID")
		req.ID = *cmd.ID
	}
	if cmd.IsMailAttempted != nil {
		conditions = append(conditions, "IsMailAttempted")
		req.IsMailAttempted = *cmd.IsMailAttempted
	}
	if cmd.Status != nil {
		conditions = append(conditions, "Status")
		req.Status = *cmd.Status
	}

	if len(conditions) == 0 {
		if err := r.db.WithContext(ctx).Find(&results); err.Error != nil {
			return nil
		}
	} else {
		if err := r.db.WithContext(ctx).Where(&req, conditions).Find(&results); err.Error != nil {
			return nil
		}
	}

	return results
}
