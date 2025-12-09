package repositories

import (
	"context"
	"fmt"
	"socket/models"
	"time"

	"gorm.io/gorm"
)

type FriendRepository interface {
	Create(ctx context.Context, friend *models.Friend) error
	GetByID(ctx context.Context, id string) (*models.Friend, error)
	GetFriendship(ctx context.Context, userID, friendID string) (*models.Friend, error)
	GetFriendships(ctx context.Context, userID string) ([]*models.Friend, error)
	GetAcceptedFriends(ctx context.Context, userID string) ([]*models.Friend, error)
	GetPendingRequests(ctx context.Context, userID string) ([]*models.Friend, error)
	Update(ctx context.Context, friend *models.Friend) error
	Delete(ctx context.Context, id string) error
	DeleteFriendship(ctx context.Context, userID, friendID string) error
	DeleteByUserID(ctx context.Context, userID string) error
	BlockUser(ctx context.Context, userID, blockUserID string) error
	IsBlocked(ctx context.Context, userID, targetUserID string) (bool, error)
	GetBlockedUsers(ctx context.Context, userID string) ([]string, error)
}

type friendRepository struct {
	db *gorm.DB
}

func NewFriendRepository(db *gorm.DB) FriendRepository {
	return &friendRepository{db: db}
}

func (r *friendRepository) Create(ctx context.Context, friend *models.Friend) error {
	// 检查是否已存在相同关系
	var existing models.Friend
	err := r.db.WithContext(ctx).
		Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
			friend.UserID, friend.FriendID, friend.FriendID, friend.UserID).
		First(&existing).Error

	if err == nil {
		// 已存在关系
		if existing.Status == "blocked" {
			return fmt.Errorf("存在屏蔽关系，无法添加好友")
		}
		return fmt.Errorf("好友关系已存在")
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("检查好友关系失败: %w", err)
	}

	// 设置创建时间
	if friend.CreatedAt.IsZero() {
		friend.CreatedAt = time.Now()
	}

	return r.db.WithContext(ctx).Create(friend).Error
}

func (r *friendRepository) GetByID(ctx context.Context, id string) (*models.Friend, error) {
	var friend models.Friend
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&friend).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询好友关系失败: %w", err)
	}
	return &friend, nil
}

func (r *friendRepository) GetFriendship(ctx context.Context, userID, friendID string) (*models.Friend, error) {
	var friend models.Friend
	err := r.db.WithContext(ctx).
		Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
			userID, friendID, friendID, userID).
		First(&friend).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询好友关系失败: %w", err)
	}
	return &friend, nil
}

func (r *friendRepository) GetFriendships(ctx context.Context, userID string) ([]*models.Friend, error) {
	var friendships []*models.Friend
	err := r.db.WithContext(ctx).
		Where("user_id = ? OR friend_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&friendships).Error

	if err != nil {
		return nil, fmt.Errorf("查询好友关系列表失败: %w", err)
	}
	return friendships, nil
}

func (r *friendRepository) GetAcceptedFriends(ctx context.Context, userID string) ([]*models.Friend, error) {
	var friends []*models.Friend
	err := r.db.WithContext(ctx).
		Where("(user_id = ? OR friend_id = ?) AND status = 'accepted'", userID, userID).
		Order("accepted_at DESC").
		Find(&friends).Error

	if err != nil {
		return nil, fmt.Errorf("查询已接受好友失败: %w", err)
	}
	return friends, nil
}

func (r *friendRepository) GetPendingRequests(ctx context.Context, userID string) ([]*models.Friend, error) {
	var requests []*models.Friend
	err := r.db.WithContext(ctx).
		Where("friend_id = ? AND status = 'pending'", userID).
		Order("created_at DESC").
		Find(&requests).Error

	if err != nil {
		return nil, fmt.Errorf("查询待处理请求失败: %w", err)
	}
	return requests, nil
}

func (r *friendRepository) Update(ctx context.Context, friend *models.Friend) error {
	return r.db.WithContext(ctx).Save(friend).Error
}

func (r *friendRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Friend{}, "id = ?", id).Error
}

func (r *friendRepository) DeleteFriendship(ctx context.Context, userID, friendID string) error {
	// 删除双向关系
	return r.db.WithContext(ctx).
		Delete(&models.Friend{},
			"(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
			userID, friendID, friendID, userID).Error
}

func (r *friendRepository) DeleteByUserID(ctx context.Context, userID string) error {
	// 删除用户的所有好友关系
	return r.db.WithContext(ctx).
		Delete(&models.Friend{}, "user_id = ? OR friend_id = ?", userID, userID).Error
}

func (r *friendRepository) BlockUser(ctx context.Context, userID, blockUserID string) error {
	// 检查是否已存在关系
	existing, err := r.GetFriendship(ctx, userID, blockUserID)
	if err != nil {
		return err
	}

	if existing != nil {
		// 更新现有关系为屏蔽
		existing.Status = "blocked"
		return r.Update(ctx, existing)
	}

	// 创建新的屏蔽关系
	friend := &models.Friend{
		ID:        fmt.Sprintf("block_%s_%s", userID[:8], blockUserID[:8]),
		UserID:    userID,
		FriendID:  blockUserID,
		Status:    "blocked",
		CreatedAt: time.Now(),
	}

	return r.Create(ctx, friend)
}

func (r *friendRepository) IsBlocked(ctx context.Context, userID, targetUserID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Friend{}).
		Where("((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)) AND status = 'blocked'",
			userID, targetUserID, targetUserID, userID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("检查屏蔽状态失败: %w", err)
	}

	return count > 0, nil
}

func (r *friendRepository) GetBlockedUsers(ctx context.Context, userID string) ([]string, error) {
	var blockedIDs []string

	// 我屏蔽的人
	var blockedByMe []string
	err := r.db.WithContext(ctx).
		Model(&models.Friend{}).
		Select("friend_id").
		Where("user_id = ? AND status = 'blocked'", userID).
		Pluck("friend_id", &blockedByMe).Error

	if err != nil {
		return nil, fmt.Errorf("查询屏蔽列表失败: %w", err)
	}

	// 屏蔽我的人
	var blockedMe []string
	err = r.db.WithContext(ctx).
		Model(&models.Friend{}).
		Select("user_id").
		Where("friend_id = ? AND status = 'blocked'", userID).
		Pluck("user_id", &blockedMe).Error

	if err != nil {
		return nil, fmt.Errorf("查询被屏蔽列表失败: %w", err)
	}

	// 合并结果
	blockedIDs = append(blockedIDs, blockedByMe...)
	blockedIDs = append(blockedIDs, blockedMe...)

	return blockedIDs, nil
}
