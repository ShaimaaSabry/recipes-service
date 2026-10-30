// Package sql stores the auth module's data in Postgres.
package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type UserSqlRepository struct {
	db *gorm.DB
}

func NewUserSqlRepository(db *gorm.DB) *UserSqlRepository {
	return &UserSqlRepository{db: db}
}

// errIdentityTaken rolls back a new account when another request created one for the same
// identity first.
var errIdentityTaken = errors.New("identity taken")

func (r *UserSqlRepository) FindOrCreateUser(ctx context.Context, identity model.Identity, newUser *model.User) (*model.User, bool, error) {
	if user, err := r.findByIdentity(ctx, identity); err != nil || user != nil {
		return user, false, err
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toUserDBO(newUser)).Error; err != nil {
			return err
		}
		inserted := tx.Exec(
			`INSERT INTO user_identities (provider, subject, user_id, email) VALUES (?, ?, ?, ?)
			 ON CONFLICT (provider, subject) DO NOTHING`,
			string(identity.Provider), identity.Subject, newUser.ID(), identity.Email,
		)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 0 {
			return errIdentityTaken
		}
		return nil
	})
	switch {
	case err == nil:
		return newUser, true, nil
	case errors.Is(err, errIdentityTaken):
		user, err := r.findByIdentity(ctx, identity)
		return user, false, err
	default:
		return nil, false, fmt.Errorf("create user: %w", err)
	}
}

func (r *UserSqlRepository) findByIdentity(ctx context.Context, identity model.Identity) (*model.User, error) {
	var dbos []userDBO
	err := r.db.WithContext(ctx).
		Joins("JOIN user_identities ON user_identities.user_id = users.id").
		Where("user_identities.provider = ? AND user_identities.subject = ?", string(identity.Provider), identity.Subject).
		Limit(1).Find(&dbos).Error
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if len(dbos) == 0 {
		return nil, nil
	}
	return toUser(dbos[0]), nil
}

func (r *UserSqlRepository) FindUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var dbos []userDBO
	if err := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if len(dbos) == 0 {
		return nil, nil
	}
	return toUser(dbos[0]), nil
}

func (r *UserSqlRepository) UserExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	if err := r.db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM users WHERE id = ?)", id).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check user: %w", err)
	}
	return exists, nil
}

// DeleteUser also deletes the user's identities and refresh tokens (ON DELETE CASCADE).
func (r *UserSqlRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&userDBO{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// SaveRefreshToken also clears the user's expired tokens, so the table doesn't grow forever.
func (r *UserSqlRepository) SaveRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND expires_at < ?", token.UserID(), time.Now()).Delete(&refreshTokenDBO{}).Error; err != nil {
			return fmt.Errorf("clear expired refresh tokens: %w", err)
		}
		dbo := refreshTokenDBO{
			ID:        token.ID(),
			UserID:    token.UserID(),
			FamilyID:  token.FamilyID(),
			TokenHash: token.Hash(),
			ExpiresAt: token.ExpiresAt(),
		}
		if err := tx.Create(&dbo).Error; err != nil {
			return fmt.Errorf("save refresh token: %w", err)
		}
		return nil
	})
}

func (r *UserSqlRepository) FindRefreshToken(ctx context.Context, hash []byte) (*model.RefreshToken, error) {
	var dbos []refreshTokenDBO
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).Limit(1).Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	if len(dbos) == 0 {
		return nil, nil
	}
	d := dbos[0]
	return model.RefreshTokenOf(d.ID, d.UserID, d.FamilyID, d.TokenHash, d.ExpiresAt, d.UsedAt, d.RevokedAt), nil
}

// MarkRefreshTokenUsed only marks a token that is still unused, so of two requests racing with the
// same token exactly one wins.
func (r *UserSqlRepository) MarkRefreshTokenUsed(ctx context.Context, id uuid.UUID, at time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&refreshTokenDBO{}).
		Where("id = ? AND used_at IS NULL AND revoked_at IS NULL", id).
		Update("used_at", at)
	if result.Error != nil {
		return false, fmt.Errorf("use refresh token: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *UserSqlRepository) RevokeRefreshTokenFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error {
	err := r.db.WithContext(ctx).Model(&refreshTokenDBO{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", at).Error
	if err != nil {
		return fmt.Errorf("revoke refresh tokens: %w", err)
	}
	return nil
}

func toUserDBO(user *model.User) *userDBO {
	return &userDBO{ID: user.ID(), Email: user.Email(), Name: user.Name(), CreatedAt: user.CreatedAt(), UpdatedAt: user.CreatedAt()}
}

func toUser(dbo userDBO) *model.User {
	return model.Of(dbo.ID, dbo.Email, dbo.Name, dbo.CreatedAt)
}
