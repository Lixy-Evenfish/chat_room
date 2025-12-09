package repositories

import (
	"context"
	"fmt"
	"socket/models"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type SessionRepository interface {
	// 会话基础操作
	Create(ctx context.Context, session *models.UserSession) error
	GetByID(ctx context.Context, id string) (*models.UserSession, error)
	GetByToken(ctx context.Context, tokenHash string) (*models.UserSession, error)
	GetByRefreshToken(ctx context.Context, refreshToken string) (*models.UserSession, error)
	GetByUserID(ctx context.Context, userID string) ([]*models.UserSession, error)
	Update(ctx context.Context, session *models.UserSession) error
	UpdateFields(ctx context.Context, id string, updates map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	DeleteByToken(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteByRefreshToken(ctx context.Context, refreshToken string) error

	// 会话管理
	CleanExpiredSessions(ctx context.Context) (int64, error)
	InvalidateUserSessions(ctx context.Context, userID string, keepCurrentToken string) error
	RevokeAllSessions(ctx context.Context) error
	GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error)
	GetSessionStats(ctx context.Context) (*models.SessionStats, error)

	// 令牌管理
	IsTokenRevoked(ctx context.Context, tokenHash string) (bool, error)
	RevokeToken(ctx context.Context, tokenHash string) error
	RevokeAllTokensForUser(ctx context.Context, userID string) error

	// 会话安全
	CheckConcurrentSessions(ctx context.Context, userID string, maxConcurrent int) (bool, error)
	DetectSuspiciousActivity(ctx context.Context, userID, ipAddress string) (bool, error)
}

type sessionRepository struct {
	db          *gorm.DB
	redisClient *redis.Client
	cachePrefix string
	cacheTTL    time.Duration
}

func NewSessionRepository(db *gorm.DB, redisClient *redis.Client) SessionRepository {
	return &sessionRepository{
		db:          db,
		redisClient: redisClient,
		cachePrefix: "session:",
		cacheTTL:    15 * time.Minute, // 缓存15分钟
	}
}

// ==================== 基础CRUD操作 ====================

func (r *sessionRepository) Create(ctx context.Context, session *models.UserSession) error {
	// 设置创建时间
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	// 使用事务确保数据一致性
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 保存到数据库
		if err := tx.Create(session).Error; err != nil {
			return fmt.Errorf("创建会话失败: %w", err)
		}

		// 保存到Redis缓存
		if r.redisClient != nil {
			cacheKey := r.getCacheKey(session.ID)
			if err := r.redisClient.Set(ctx, cacheKey, session.Token, r.cacheTTL).Err(); err != nil {
				// 如果缓存失败，记录日志但不影响主流程
				fmt.Printf("缓存会话失败: %v\n", err)
			}
		}

		return nil
	})
}

func (r *sessionRepository) GetByID(ctx context.Context, id string) (*models.UserSession, error) {
	var session models.UserSession

	// 先尝试从缓存获取
	if r.redisClient != nil {
		cacheKey := r.getCacheKey(id)
		cachedToken, err := r.redisClient.Get(ctx, cacheKey).Result()
		if err == nil && cachedToken != "" {
			// 缓存命中，从数据库获取完整信息
			err = r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
			if err == nil {
				return &session, nil
			}
		}
	}

	// 从数据库获取
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}

	// 存入缓存
	if r.redisClient != nil {
		cacheKey := r.getCacheKey(id)
		r.redisClient.Set(ctx, cacheKey, session.Token, r.cacheTTL)
	}

	return &session, nil
}

func (r *sessionRepository) GetByToken(ctx context.Context, tokenHash string) (*models.UserSession, error) {
	var session models.UserSession

	// 优先从数据库查询（令牌变化频繁，缓存意义不大）
	err := r.db.WithContext(ctx).
		Where("token = ?", tokenHash).
		Where("expires_at > ?", time.Now()).
		First(&session).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}

	// 检查是否被撤销
	revoked, _ := r.IsTokenRevoked(ctx, tokenHash)
	if revoked {
		return nil, nil
	}

	return &session, nil
}

func (r *sessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*models.UserSession, error) {
	var session models.UserSession

	err := r.db.WithContext(ctx).
		Where("refresh_token = ?", refreshToken).
		Where("expires_at > ?", time.Now()).
		First(&session).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}

	return &session, nil
}

func (r *sessionRepository) GetByUserID(ctx context.Context, userID string) ([]*models.UserSession, error) {
	var sessions []*models.UserSession

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Where("expires_at > ?", time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error

	if err != nil {
		return nil, fmt.Errorf("查询用户会话失败: %w", err)
	}

	return sessions, nil
}

func (r *sessionRepository) Update(ctx context.Context, session *models.UserSession) error {
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return fmt.Errorf("更新会话失败: %w", err)
	}

	// 更新缓存
	if r.redisClient != nil {
		cacheKey := r.getCacheKey(session.ID)
		r.redisClient.Del(ctx, cacheKey) // 删除缓存，下次访问重新加载
	}

	return nil
}

func (r *sessionRepository) UpdateFields(ctx context.Context, id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	if err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("更新会话字段失败: %w", err)
	}

	// 清除缓存
	if r.redisClient != nil {
		cacheKey := r.getCacheKey(id)
		r.redisClient.Del(ctx, cacheKey)
	}

	return nil
}

func (r *sessionRepository) Delete(ctx context.Context, id string) error {
	// 先获取会话信息（用于清理令牌）
	session, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if session != nil {
		// 标记令牌为撤销状态
		r.RevokeToken(ctx, session.Token)
	}

	// 从数据库删除
	if err := r.db.WithContext(ctx).Delete(&models.UserSession{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
	}

	// 清除缓存
	if r.redisClient != nil {
		cacheKey := r.getCacheKey(id)
		r.redisClient.Del(ctx, cacheKey)
	}

	return nil
}

func (r *sessionRepository) DeleteByToken(ctx context.Context, tokenHash string) error {
	// 获取会话ID
	var sessionID string
	err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Select("id").
		Where("token = ?", tokenHash).
		Pluck("id", &sessionID).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询会话ID失败: %w", err)
	}

	// 标记令牌为撤销状态
	r.RevokeToken(ctx, tokenHash)

	// 删除会话
	if err := r.db.WithContext(ctx).Delete(&models.UserSession{}, "token = ?", tokenHash).Error; err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
	}

	// 清除缓存
	if sessionID != "" && r.redisClient != nil {
		cacheKey := r.getCacheKey(sessionID)
		r.redisClient.Del(ctx, cacheKey)
	}

	return nil
}

func (r *sessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	// 获取该用户的所有会话令牌
	var tokens []string
	err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Select("token").
		Where("user_id = ?", userID).
		Pluck("token", &tokens).Error

	if err != nil {
		return fmt.Errorf("查询用户令牌失败: %w", err)
	}

	// 批量撤销令牌
	for _, token := range tokens {
		r.RevokeToken(ctx, token)
	}

	// 从数据库删除所有会话
	if err := r.db.WithContext(ctx).Delete(&models.UserSession{}, "user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("删除用户会话失败: %w", err)
	}

	return nil
}

func (r *sessionRepository) DeleteByRefreshToken(ctx context.Context, refreshToken string) error {
	// 获取会话信息
	session, err := r.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	if session != nil {
		// 标记令牌为撤销状态
		r.RevokeToken(ctx, session.Token)

		// 删除会话
		if err := r.db.WithContext(ctx).Delete(&models.UserSession{}, "refresh_token = ?", refreshToken).Error; err != nil {
			return fmt.Errorf("删除会话失败: %w", err)
		}

		// 清除缓存
		if r.redisClient != nil {
			cacheKey := r.getCacheKey(session.ID)
			r.redisClient.Del(ctx, cacheKey)
		}
	}

	return nil
}

// ==================== 会话管理 ====================

func (r *sessionRepository) CleanExpiredSessions(ctx context.Context) (int64, error) {
	// 删除所有已过期的会话
	result := r.db.WithContext(ctx).
		Where("expires_at <= ?", time.Now()).
		Delete(&models.UserSession{})

	if result.Error != nil {
		return 0, fmt.Errorf("清理过期会话失败: %w", result.Error)
	}

	// 清理Redis中的过期令牌
	if r.redisClient != nil {
		// 这里可以添加Redis清理逻辑
		// 例如：使用SCAN命令找到并删除过期的会话缓存
	}

	return result.RowsAffected, nil
}

func (r *sessionRepository) InvalidateUserSessions(ctx context.Context, userID string, keepCurrentToken string) error {
	var condition string
	var args []interface{}

	if keepCurrentToken != "" {
		// 保留当前令牌，删除其他所有会话
		condition = "user_id = ? AND token != ?"
		args = []interface{}{userID, keepCurrentToken}
	} else {
		// 删除用户的所有会话
		condition = "user_id = ?"
		args = []interface{}{userID}
	}

	// 获取要删除的会话令牌
	var tokens []string
	err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Select("token").
		Where(condition, args...).
		Pluck("token", &tokens).Error

	if err != nil {
		return fmt.Errorf("查询会话令牌失败: %w", err)
	}

	// 批量撤销令牌（除了保留的令牌）
	for _, token := range tokens {
		if token != keepCurrentToken {
			r.RevokeToken(ctx, token)
		}
	}

	// 删除会话记录
	if err := r.db.WithContext(ctx).
		Where(condition, args...).
		Delete(&models.UserSession{}).Error; err != nil {
		return fmt.Errorf("使会话失效失败: %w", err)
	}

	return nil
}

func (r *sessionRepository) RevokeAllSessions(ctx context.Context) error {
	// 获取所有会话令牌
	var tokens []string
	err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Select("token").
		Pluck("token", &tokens).Error

	if err != nil {
		return fmt.Errorf("查询所有令牌失败: %w", err)
	}

	// 批量撤销所有令牌
	for _, token := range tokens {
		r.RevokeToken(ctx, token)
	}

	// 删除所有会话记录
	if err := r.db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).
		Delete(&models.UserSession{}).Error; err != nil {
		return fmt.Errorf("撤销所有会话失败: %w", err)
	}

	// 清理Redis缓存
	if r.redisClient != nil {
		// 查找所有会话缓存并删除
		iter := r.redisClient.Scan(ctx, 0, r.cachePrefix+"*", 0).Iterator()
		for iter.Next(ctx) {
			r.redisClient.Del(ctx, iter.Val())
		}
	}

	return nil
}

func (r *sessionRepository) GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	var sessions []*models.UserSession

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Where("expires_at > ?", time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error

	if err != nil {
		return nil, fmt.Errorf("获取活跃会话失败: %w", err)
	}

	// 过滤已被撤销的令牌
	activeSessions := make([]*models.UserSession, 0, len(sessions))
	for _, session := range sessions {
		revoked, _ := r.IsTokenRevoked(ctx, session.Token)
		if !revoked {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions, nil
}

func (r *sessionRepository) GetSessionStats(ctx context.Context) (*models.SessionStats, error) {
	var stats models.SessionStats

	// 获取总会话数
	if err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Count(&stats.TotalSessions).Error; err != nil {
		return nil, fmt.Errorf("统计总会话数失败: %w", err)
	}

	// 获取活跃会话数（未过期）
	if err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Where("expires_at > ?", time.Now()).
		Count(&stats.ActiveSessions).Error; err != nil {
		return nil, fmt.Errorf("统计活跃会话数失败: %w", err)
	}

	// 获取过期会话数
	if err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Where("expires_at <= ?", time.Now()).
		Count(&stats.ExpiredSessions).Error; err != nil {
		return nil, fmt.Errorf("统计过期会话数失败: %w", err)
	}

	// 获取唯一用户数
	var uniqueUsers int64
	if err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Select("COUNT(DISTINCT user_id)").
		Scan(&uniqueUsers).Error; err != nil {
		return nil, fmt.Errorf("统计唯一用户数失败: %w", err)
	}
	stats.UniqueUsers = uniqueUsers

	// 计算平均每个用户的会话数
	if uniqueUsers > 0 {
		stats.AvgSessionsPerUser = stats.TotalSessions / uniqueUsers
	}

	return &stats, nil
}

// ==================== 令牌管理 ====================

func (r *sessionRepository) IsTokenRevoked(ctx context.Context, tokenHash string) (bool, error) {
	if r.redisClient == nil {
		// 如果没有Redis，检查数据库中的撤销记录
		var count int64
		err := r.db.WithContext(ctx).
			Model(&models.TokenBlacklist{}).
			Where("token_hash = ?", tokenHash).
			Count(&count).Error

		if err != nil {
			return false, fmt.Errorf("检查令牌状态失败: %w", err)
		}

		return count > 0, nil
	}

	// 使用Redis检查令牌是否被撤销
	revokedKey := fmt.Sprintf("revoked:%s", tokenHash)
	result, err := r.redisClient.Exists(ctx, revokedKey).Result()
	if err != nil {
		return false, fmt.Errorf("检查令牌撤销状态失败: %w", err)
	}

	return result > 0, nil
}

func (r *sessionRepository) RevokeToken(ctx context.Context, tokenHash string) error {
	if r.redisClient == nil {
		// 如果没有Redis，记录到数据库
		blacklist := &models.TokenBlacklist{
			ID:        fmt.Sprintf("bl_%s", tokenHash[:16]),
			TokenHash: tokenHash,
			RevokedAt: time.Now(),
		}

		return r.db.WithContext(ctx).Create(blacklist).Error
	}

	// 使用Redis记录撤销的令牌（设置过期时间，自动清理）
	revokedKey := fmt.Sprintf("revoked:%s", tokenHash)
	// 设置过期时间为7天，确保过期令牌不会永久占用内存
	expiry := 7 * 24 * time.Hour

	return r.redisClient.Set(ctx, revokedKey, "1", expiry).Err()
}

func (r *sessionRepository) RevokeAllTokensForUser(ctx context.Context, userID string) error {
	// 获取用户的所有令牌
	var tokens []string
	err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Select("token").
		Where("user_id = ?", userID).
		Pluck("token", &tokens).Error

	if err != nil {
		return fmt.Errorf("获取用户令牌失败: %w", err)
	}

	// 批量撤销令牌
	for _, token := range tokens {
		if err := r.RevokeToken(ctx, token); err != nil {
			// 记录错误但继续处理其他令牌
			fmt.Printf("撤销令牌失败: %v\n", err)
		}
	}

	return nil
}

// ==================== 会话安全 ====================

func (r *sessionRepository) CheckConcurrentSessions(ctx context.Context, userID string, maxConcurrent int) (bool, error) {
	// 获取用户的活跃会话数
	var activeSessions int64
	err := r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Where("user_id = ?", userID).
		Where("expires_at > ?", time.Now()).
		Count(&activeSessions).Error

	if err != nil {
		return false, fmt.Errorf("检查并发会话失败: %w", err)
	}

	// 检查是否超过最大并发限制
	if maxConcurrent > 0 && activeSessions >= int64(maxConcurrent) {
		return false, nil // 不允许新会话
	}

	return true, nil // 允许新会话
}

func (r *sessionRepository) DetectSuspiciousActivity(ctx context.Context, userID, ipAddress string) (bool, error) {
	// 获取用户最近的活动
	var recentSessions []*models.UserSession
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Where("created_at > ?", oneHourAgo).
		Order("created_at DESC").
		Find(&recentSessions).Error

	if err != nil {
		return false, fmt.Errorf("检测可疑活动失败: %w", err)
	}

	// 分析可疑活动模式
	suspicious := false

	// 1. 检查短时间内是否从多个不同IP登录
	uniqueIPs := make(map[string]bool)
	for _, session := range recentSessions {
		uniqueIPs[session.IPAddress] = true
	}

	// 如果一小时内从超过3个不同IP登录，视为可疑
	if len(uniqueIPs) > 3 {
		suspicious = true
	}

	// 2. 检查是否有异常的地理位置变化（需要IP地理位置数据库）
	// 这里可以集成第三方IP地理位置服务

	// 3. 检查是否频繁创建会话（超过5次/小时）
	if len(recentSessions) > 5 {
		suspicious = true
	}

	return suspicious, nil
}

// ==================== 辅助方法 ====================

func (r *sessionRepository) getCacheKey(id string) string {
	return r.cachePrefix + id
}

// 清理过期的撤销令牌（定期任务）
func (r *sessionRepository) CleanExpiredRevokedTokens(ctx context.Context) error {
	if r.redisClient != nil {
		// Redis中的撤销令牌设置了过期时间，会自动清理
		return nil
	}

	// 清理数据库中的过期撤销令牌（保留最近7天）
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

	result := r.db.WithContext(ctx).
		Where("revoked_at <= ?", sevenDaysAgo).
		Delete(&models.TokenBlacklist{})

	if result.Error != nil {
		return fmt.Errorf("清理过期撤销令牌失败: %w", result.Error)
	}

	return nil
}
