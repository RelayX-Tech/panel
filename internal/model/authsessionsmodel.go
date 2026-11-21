package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AuthSessionsModel = (*customAuthSessionsModel)(nil)

type (
	// AuthSessionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthSessionsModel.
	AuthSessionsModel interface {
		authSessionsModel
	}

	customAuthSessionsModel struct {
		*defaultAuthSessionsModel
	}
)

// NewAuthSessionsModel returns a model for the database table.
func NewAuthSessionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AuthSessionsModel {
	return &customAuthSessionsModel{
		defaultAuthSessionsModel: newAuthSessionsModel(conn, c, opts...),
	}
}
