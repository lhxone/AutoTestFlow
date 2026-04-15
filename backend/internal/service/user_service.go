package service

import (
	"errors"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo         *repository.UserRepo
	operationLogRepo *repository.OperationLogRepo
}

func NewUserService() *UserService {
	return &UserService{
		userRepo:         repository.NewUserRepo(),
		operationLogRepo: repository.NewOperationLogRepo(),
	}
}

// Create 创建用户
func (s *UserService) Create(req *dto.CreateUserRequest) (*model.User, error) {
	// 检查用户名是否重复
	if existing, _ := s.userRepo.GetByUsername(req.Username); existing != nil {
		return nil, errors.New("用户名已存在")
	}

	// 加密密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	user := &model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		RealName:     req.RealName,
		Email:        req.Email,
		Phone:        req.Phone,
		Status:       1,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// 设置角色
	if len(req.RoleIDs) > 0 {
		if err := s.userRepo.SetRoles(user.ID, req.RoleIDs); err != nil {
			return nil, err
		}
	}

	return s.userRepo.GetByID(user.ID)
}

// GetByID 获取用户详情
func (s *UserService) GetByID(id uint64) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

// Update 更新用户
func (s *UserService) Update(id uint64, req *dto.UpdateUserRequest) (*model.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	if req.RealName != "" {
		user.RealName = req.RealName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	if len(req.RoleIDs) > 0 {
		if err := s.userRepo.SetRoles(id, req.RoleIDs); err != nil {
			return nil, err
		}
	}

	return s.userRepo.GetByID(id)
}

// Delete 删除用户(软删除)
func (s *UserService) Delete(id uint64) error {
	return s.userRepo.Delete(id)
}

// List 用户列表
func (s *UserService) List(req *dto.UserListQuery) ([]model.User, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize
	return s.userRepo.List(req.Keyword, req.Status, req.RoleCode, offset, req.PageSize)
}

func (s *UserService) ListLoginLogs(req *dto.LoginLogListQuery) ([]model.OperationLog, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize
	return s.operationLogRepo.ListAuthLogs(req.Username, req.Action, offset, req.PageSize)
}
