package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo() *UserRepo {
	return &UserRepo{db: DB}
}

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) GetByID(id uint64) (*model.User, error) {
	var user model.User
	err := r.db.Preload("Roles").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Preload("Roles").Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepo) Delete(id uint64) error {
	return r.db.Delete(&model.User{}, id).Error
}

func (r *UserRepo) List(keyword string, status *int8, roleCode string, offset, limit int) ([]model.User, int64, error) {
	query := r.db.Model(&model.User{}).Preload("Roles")

	if keyword != "" {
		query = query.Where("username LIKE ? OR real_name LIKE ? OR email LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if roleCode != "" {
		query = query.Joins("JOIN user_role ON user_role.user_id = user.id").
			Joins("JOIN role ON role.id = user_role.role_id").
			Where("role.code = ?", roleCode)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// UpdateLastLogin 更新最后登录时间
func (r *UserRepo) UpdateLastLogin(id uint64) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("last_login_at", gorm.Expr("NOW()")).Error
}

// SetRoles 设置用户角色
func (r *UserRepo) SetRoles(userID uint64, roleIDs []uint64) error {
	// 先删除旧角色
	if err := r.db.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return err
	}
	// 再插入新角色
	for _, roleID := range roleIDs {
		ur := model.UserRole{UserID: userID, RoleID: roleID}
		if err := r.db.Create(&ur).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetUserPermissions 获取用户的所有权限code
func (r *UserRepo) GetUserPermissions(userID uint64) ([]string, error) {
	var codes []string
	err := r.db.Model(&model.Permission{}).
		Joins("JOIN role_permission ON role_permission.permission_id = permission.id").
		Joins("JOIN user_role ON user_role.role_id = role_permission.role_id").
		Where("user_role.user_id = ?", userID).
		Pluck("permission.code", &codes).Error
	return codes, err
}
