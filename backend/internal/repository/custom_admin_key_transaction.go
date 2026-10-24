package repository

import (
	"context"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ScopedAdminLockAccount preserves the protection writers' plan -> account lock
// order before scoped authorization takes its account and membership row locks.
func (r *accountRepository) ScopedAdminLockAccount(ctx context.Context, id int64) error {
	tx := dbent.TxFromContext(ctx)
	if tx == nil {
		return fmt.Errorf("受限账户锁需要数据库事务")
	}
	_, err := lockAccountMembershipProtection(ctx, tx.Client(), id)
	return translatePersistenceError(err, service.ErrAccountNotFound, nil)
}

// accountTransactionScope 同时切换 ORM、原生 SQL 和关联查询，禁止在提交前发布缓存。
func (r *accountRepository) accountTransactionScope(ctx context.Context) *accountRepository {
	tx := dbent.TxFromContext(ctx)
	if tx == nil || r.client == tx.Client() {
		return r
	}
	scoped := *r
	scoped.client = tx.Client()
	scoped.sql = tx.Client()
	scoped.schedulerCache = nil
	return &scoped
}
