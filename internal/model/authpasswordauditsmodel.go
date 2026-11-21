package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AuthPasswordAuditsModel = (*customAuthPasswordAuditsModel)(nil)

type (
	// AuthPasswordAuditsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthPasswordAuditsModel.
	AuthPasswordAuditsModel interface {
		authPasswordAuditsModel
		withSession(session sqlx.Session) AuthPasswordAuditsModel
	}

	customAuthPasswordAuditsModel struct {
		*defaultAuthPasswordAuditsModel
	}
)

// NewAuthPasswordAuditsModel returns a model for the database table.
func NewAuthPasswordAuditsModel(conn sqlx.SqlConn) AuthPasswordAuditsModel {
	return &customAuthPasswordAuditsModel{
		defaultAuthPasswordAuditsModel: newAuthPasswordAuditsModel(conn),
	}
}

func (m *customAuthPasswordAuditsModel) withSession(session sqlx.Session) AuthPasswordAuditsModel {
	return NewAuthPasswordAuditsModel(sqlx.NewSqlConnFromSession(session))
}
