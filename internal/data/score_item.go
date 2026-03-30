package data

import (
	"context"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type scoreItemRepo struct {
	data *Data
}

func NewScoreItemRepo(data *Data) biz.ScoreItemRepo {
	return &scoreItemRepo{
		data: data,
	}
}

func (r *scoreItemRepo) ListByScoreID(ctx context.Context, scoreID int64) ([]*biz.ScoreItem, error) {
	var items []*biz.ScoreItem
	err := r.data.db.WithContext(ctx).Where("score_id = ?", scoreID).Find(&items).Error
	if err != nil {
		log.Context(ctx).Errorf("query scoreItemRepo.ListByScoreID err: %+v", err)
		return nil, err
	}
	return items, nil
}
