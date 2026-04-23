package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ZentaoTestCaseRepo struct {
	db *gorm.DB
}

func NewZentaoTestCaseRepo() *ZentaoTestCaseRepo {
	return &ZentaoTestCaseRepo{db: DB}
}

// Upsert 插入或更新用例(按 zentao_id + product_id 判重)
func (r *ZentaoTestCaseRepo) Upsert(tc *model.ZentaoTestCase) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "zentao_id"}, {Name: "product_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "precondition", "keywords", "priority", "type", "stage",
			"status", "test_status", "branch", "module", "steps", "expected",
			"opened_by", "created_by", "synced_at", "zentao_updated_at",
		}),
	}).Create(tc).Error
}

func (r *ZentaoTestCaseRepo) GetByID(id uint64) (*model.ZentaoTestCase, error) {
	var tc model.ZentaoTestCase
	err := r.db.First(&tc, id).Error
	if err != nil {
		return nil, err
	}
	return &tc, nil
}

func (r *ZentaoTestCaseRepo) List(projectID, productID uint64, testStatus, branch, keyword, tcType string, offset, limit int) ([]model.ZentaoTestCase, int64, error) {
	query := r.db.Model(&model.ZentaoTestCase{})

	if projectID > 0 {
		query = query.Where("product_id = ?", projectID)
	}
	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	if testStatus != "" {
		query = query.Where("test_status = ?", testStatus)
	}
	if branch != "" {
		query = query.Where("branch = ?", branch)
	}
	if tcType != "" {
		query = query.Where("type = ?", tcType)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR precondition LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var cases []model.ZentaoTestCase
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&cases).Error; err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}

func (r *ZentaoTestCaseRepo) FindByProductAndZentaoIDs(productID uint64, zentaoIDs []int) ([]model.ZentaoTestCase, error) {
	if len(zentaoIDs) == 0 {
		return []model.ZentaoTestCase{}, nil
	}

	var cases []model.ZentaoTestCase
	err := r.db.Where("product_id = ? AND zentao_id IN ?", productID, zentaoIDs).Find(&cases).Error
	return cases, err
}

func (r *ZentaoTestCaseRepo) DeleteMissingByProduct(productID uint64, keepZentaoIDs []int) (int64, error) {
	query := r.db.Where("product_id = ?", productID)
	if len(keepZentaoIDs) > 0 {
		query = query.Where("zentao_id NOT IN ?", keepZentaoIDs)
	}

	result := query.Delete(&model.ZentaoTestCase{})
	return result.RowsAffected, result.Error
}

// FindMissingZentaoIDsByProduct 查找不在给定列表中的用例 ZentaoID
func (r *ZentaoTestCaseRepo) FindMissingZentaoIDsByProduct(productID uint64, keepZentaoIDs []int) ([]int, error) {
	query := r.db.Model(&model.ZentaoTestCase{}).Where("product_id = ?", productID)
	if len(keepZentaoIDs) > 0 {
		query = query.Where("zentao_id NOT IN ?", keepZentaoIDs)
	}

	var zentaoIDs []int
	err := query.Pluck("zentao_id", &zentaoIDs).Error
	return zentaoIDs, err
}

func (r *ZentaoTestCaseRepo) ListMissingByProduct(productID uint64, keepZentaoIDs []int) ([]model.ZentaoTestCase, error) {
	query := r.db.Where("product_id = ?", productID)
	if len(keepZentaoIDs) > 0 {
		query = query.Where("zentao_id NOT IN ?", keepZentaoIDs)
	}

	var cases []model.ZentaoTestCase
	err := query.Order("id DESC").Find(&cases).Error
	return cases, err
}

// UpdateTestStatus 更新测试状态
func (r *ZentaoTestCaseRepo) UpdateTestStatus(id uint64, newStatus string) error {
	return r.db.Model(&model.ZentaoTestCase{}).Where("id = ?", id).Update("test_status", newStatus).Error
}
