package service

import (
	"errors"
	"time"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo         *repository.UserRepo
	operationLogRepo *repository.OperationLogRepo
}

func NewAuthService() *AuthService {
	return &AuthService{
		userRepo:         repository.NewUserRepo(),
		operationLogRepo: repository.NewOperationLogRepo(),
	}
}

// Login 用户登录
func (s *AuthService) Login(req *dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	if user.Status != 1 {
		return nil, errors.New("用户已被禁用")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// 获取第一个角色code(简化处理，JWT中只存一个主角色)
	roleCode := ""
	if len(user.Roles) > 0 {
		roleCode = user.Roles[0].Code
	}

	// 生成令牌
	accessToken, err := pkg.GenerateToken(user.ID, user.Username, roleCode)
	if err != nil {
		return nil, errors.New("生成令牌失败")
	}

	refreshToken, err := pkg.GenerateRefreshToken(user.ID, user.Username, roleCode)
	if err != nil {
		return nil, errors.New("生成刷新令牌失败")
	}

	// 更新最后登录时间
	_ = s.userRepo.UpdateLastLogin(user.ID)

	// 获取权限列表
	permissions, _ := s.userRepo.GetUserPermissions(user.ID)

	// 组装角色名称列表
	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Code)
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    24 * 3600, // 24小时
		User: dto.UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			RealName:    user.RealName,
			Email:       user.Email,
			Phone:       user.Phone,
			Avatar:      user.Avatar,
			Status:      user.Status,
			Roles:       roleNames,
			Permissions: permissions,
		},
	}, nil
}

// RefreshToken 刷新令牌
func (s *AuthService) RefreshToken(req *dto.RefreshTokenRequest) (*dto.LoginResponse, error) {
	claims, err := pkg.ParseToken(req.RefreshToken)
	if err != nil {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	user, err := s.userRepo.GetByID(claims.UserID)
	if err != nil || user.Status != 1 {
		return nil, errors.New("用户不存在或已被禁用")
	}

	roleCode := ""
	if len(user.Roles) > 0 {
		roleCode = user.Roles[0].Code
	}

	accessToken, err := pkg.GenerateToken(user.ID, user.Username, roleCode)
	if err != nil {
		return nil, errors.New("生成令牌失败")
	}

	refreshToken, err := pkg.GenerateRefreshToken(user.ID, user.Username, roleCode)
	if err != nil {
		return nil, errors.New("生成刷新令牌失败")
	}

	permissions, _ := s.userRepo.GetUserPermissions(user.ID)
	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Code)
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    24 * 3600,
		User: dto.UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			RealName:    user.RealName,
			Email:       user.Email,
			Roles:       roleNames,
			Permissions: permissions,
		},
	}, nil
}

// GetCurrentUser 获取当前登录用户信息
func (s *AuthService) GetCurrentUser(userID uint64) (*dto.UserInfo, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	permissions, _ := s.userRepo.GetUserPermissions(userID)
	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Code)
	}

	return &dto.UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		RealName:    user.RealName,
		Email:       user.Email,
		Phone:       user.Phone,
		Avatar:      user.Avatar,
		Status:      user.Status,
		Roles:       roleNames,
		Permissions: permissions,
	}, nil
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(userID uint64, req *dto.ChangePasswordRequest) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return errors.New("原密码错误")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}

	user.PasswordHash = string(hash)
	_ = time.Now() // 占位，避免 import 未使用
	return s.userRepo.Update(user)
}

func (s *AuthService) CreateAuthLog(log *model.OperationLog) error {
	return s.operationLogRepo.Create(log)
}
