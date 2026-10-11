// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// UserCreateWithIdentity registers a user who signed in through an identity
// provider, together with a fresh personal org and the identity, in one
// transaction. The login comes from the identity provider and is not
// forge-verified, so it must not match any existing user or org on any forge,
// compared case-insensitively; types.ErrNameTaken is returned otherwise.
func (s storage) UserCreateWithIdentity(user *model.User, identity *model.UserIdentity) error {
	// Two registrations whose logins differ only in case must not both pass the
	// availability check, so registrations take a database-wide lock on Postgres
	// and MySQL; SQLite serializes write transactions and fails the later one.
	// Serializable isolation would also prevent this, but there unrelated
	// concurrent registrations abort each other. A transaction can still
	// deadlock with other writers, so those aborts are retried.
	var err error
	for attempt := range maxTxAttempts {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * txRetryDelay)
		}
		// A rolled-back attempt may have set IDs that no longer exist.
		user.ID, user.OrgID = 0, 0
		identity.ID, identity.UserID = 0, 0
		err = s.userCreateWithIdentity(user, identity)
		if !isTxConflict(err) {
			return err
		}
	}
	return err
}

const (
	maxTxAttempts = 3
	txRetryDelay  = 20 * time.Millisecond

	mysqlErrLockDeadlock = 1213

	// The lock that serializes registrations: an advisory lock key ("crowuser")
	// on Postgres and a named lock on MySQL.
	registrationLockKey     int64 = 0x63726f7775736572
	registrationLockName          = "crow_user_registration"
	registrationLockTimeout       = 10 // seconds
)

// isTxConflict reports whether a transaction was aborted because it conflicted
// with a concurrent one and can be retried as a whole.
func isTxConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// serialization_failure and deadlock_detected
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		return myErr.Number == mysqlErrLockDeadlock
	}
	return false
}

func (s storage) userCreateWithIdentity(user *model.User, identity *model.UserIdentity) error {
	ctx := context.Background()
	insert := func(ctx context.Context, tx bun.Tx) error {
		return insertUserWithIdentity(ctx, tx, user, identity)
	}

	switch s.driver {
	case DriverPostgres:
		return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// Released when the transaction ends.
			if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", registrationLockKey); err != nil {
				return fmt.Errorf("failed to take the registration lock: %w", err)
			}
			return insert(ctx, tx)
		})
	case DriverMysql:
		// MySQL named locks belong to the session, so take it on a dedicated
		// connection and release it only after the transaction committed.
		conn, err := s.db.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		var locked sql.NullInt64
		if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", registrationLockName, registrationLockTimeout).Scan(&locked); err != nil {
			return fmt.Errorf("failed to take the registration lock: %w", err)
		}
		if locked.Int64 != 1 {
			return errors.New("timed out waiting for the registration lock")
		}
		defer func() {
			_, _ = conn.ExecContext(context.Background(), "DO RELEASE_LOCK(?)", registrationLockName)
		}()
		return conn.RunInTx(ctx, nil, insert)
	default:
		return s.db.RunInTx(ctx, nil, insert)
	}
}

func insertUserWithIdentity(ctx context.Context, tx bun.Tx, user *model.User, identity *model.UserIdentity) error {
	name := strings.ToLower(user.Login)
	users, err := tx.NewSelect().Model((*model.User)(nil)).Where("LOWER(login) = ?", name).Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check login availability: %w", err)
	}
	orgs, err := tx.NewSelect().Model((*model.Org)(nil)).Where("LOWER(name) = ?", name).Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check org availability: %w", err)
	}
	if users > 0 || orgs > 0 {
		return types.ErrNameTaken
	}

	org := &model.Org{Name: user.Login, ForgeID: user.ForgeID, IsUser: true}
	if _, err := tx.NewInsert().Model(org).Exec(ctx); err != nil {
		return fmt.Errorf("failed to create org: %w", err)
	}
	user.OrgID = org.ID
	if _, err := tx.NewInsert().Model(user).Exec(ctx); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	identity.UserID = user.ID
	if identity.LinkedAt == 0 {
		identity.LinkedAt = time.Now().Unix()
	}
	if _, err := tx.NewInsert().Model(identity).Exec(ctx); err != nil {
		return fmt.Errorf("failed to create identity: %w", err)
	}
	return nil
}

func (s storage) UserIdentityGetByProviderSubject(provider, issuer, subject string) (*model.UserIdentity, error) {
	identity := new(model.UserIdentity)
	return identity, wrapGet(s.db.NewSelect().Model(identity).
		Where("provider = ? AND issuer = ? AND subject = ?", provider, issuer, subject).
		Scan(context.Background()))
}

func (s storage) UserIdentityList(userID int64) ([]*model.UserIdentity, error) {
	var identities []*model.UserIdentity
	return identities, s.db.NewSelect().Model(&identities).
		Where("user_id = ?", userID).
		Order("linked_at ASC").
		Scan(context.Background())
}

func (s storage) UserIdentityCreate(identity *model.UserIdentity) error {
	if identity.LinkedAt == 0 {
		identity.LinkedAt = time.Now().Unix()
	}
	_, err := s.db.NewInsert().Model(identity).Exec(context.Background())
	return err
}

func (s storage) UserIdentityUpdate(identity *model.UserIdentity) error {
	_, err := s.db.NewUpdate().Model(identity).WherePK().Exec(context.Background())
	return err
}

func (s storage) UserIdentityDelete(identity *model.UserIdentity) error {
	return wrapDelete(s.db.NewDelete().Model((*model.UserIdentity)(nil)).Where("id = ?", identity.ID).Exec(context.Background()))
}
