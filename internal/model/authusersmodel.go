package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AuthUsersModel = (*customAuthUsersModel)(nil)

type (
	// AuthUsersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthUsersModel.
	AuthUsersModel interface {
		authUsersModel
		// DelCacheCtx 删除主键点查缓存；事务内手写 UPDATE 不经过生成方法，
		// 需在提交后显式失效（design.md §4.6）。
		DelCacheCtx(ctx context.Context, keys ...string) error
	}

	customAuthUsersModel struct {
		*defaultAuthUsersModel
	}
)

// NewAuthUsersModel returns a model for the database table.
func NewAuthUsersModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AuthUsersModel {
	return &customAuthUsersModel{
		defaultAuthUsersModel: newAuthUsersModel(conn, c, opts...),
	}
}
