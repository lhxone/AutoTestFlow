package service

import (
	"errors"
	"math"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
)

type ProjectService struct {
	projectRepo *repository.ProjectRepo
	syncLogRepo *repository.IssueSyncLogRepo
}

func NewProjectService() *ProjectService {
	return &ProjectService{
		projectRepo: repository.NewProjectRepo(),
		syncLogRepo: repository.NewIssueSyncLogRepo(),
	}
}

func (s *ProjectService) Create(req *dto.CreateProjectRequest) (*model.Project, error) {
	p := &model.Project{
		Name:              req.Name,
		Description:       req.Description,
		FuncDocPath:       req.FuncDocPath,
		DesignDocPath:     req.DesignDocPath,
		DBDocPath:         req.DBDocPath,
		TestDocPath:       req.TestDocPath,
		ExtraFilesPath:    req.ExtraFilesPath,
		GitRepoURL:        req.GitRepoURL,
		GitBranch:         req.GitBranch,
		GitPullInterval:   req.GitPullInterval,
		ZentaoProjectID:   req.ZentaoProjectID,
		ZentaoProjectName: req.ZentaoProjectName,
		ZentaoBranch:      req.ZentaoBranch,
		OwnerID:           req.OwnerID,
		Status:            1,
	}
	if p.GitBranch == "" {
		p.GitBranch = "main"
	}

	if err := s.projectRepo.Create(p); err != nil {
		return nil, err
	}
	return s.projectRepo.GetByID(p.ID)
}

func (s *ProjectService) GetByID(id uint64) (*model.Project, error) {
	return s.projectRepo.GetByID(id)
}

func (s *ProjectService) Update(id uint64, req *dto.UpdateProjectRequest) (*model.Project, error) {
	p, err := s.projectRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("项目不存在")
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.FuncDocPath != "" {
		p.FuncDocPath = req.FuncDocPath
	}
	if req.DesignDocPath != "" {
		p.DesignDocPath = req.DesignDocPath
	}
	if req.DBDocPath != "" {
		p.DBDocPath = req.DBDocPath
	}
	if req.TestDocPath != "" {
		p.TestDocPath = req.TestDocPath
	}
	if req.ExtraFilesPath != "" {
		p.ExtraFilesPath = req.ExtraFilesPath
	}
	if req.GitRepoURL != "" {
		p.GitRepoURL = req.GitRepoURL
	}
	if req.GitBranch != "" {
		p.GitBranch = req.GitBranch
	}
	if req.GitPullInterval != nil {
		p.GitPullInterval = *req.GitPullInterval
	}
	p.ZentaoProjectID = req.ZentaoProjectID
	p.ZentaoProjectName = req.ZentaoProjectName
	p.ZentaoBranch = req.ZentaoBranch
	if req.Status != nil {
		p.Status = *req.Status
	}
	if req.OwnerID != nil {
		p.OwnerID = req.OwnerID
	}

	if err := s.projectRepo.Update(p); err != nil {
		return nil, err
	}
	return s.projectRepo.GetByID(id)
}

func (s *ProjectService) Delete(id uint64) error {
	return s.projectRepo.Delete(id)
}

func (s *ProjectService) List(req *dto.ProjectListQuery) ([]model.Project, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize
	return s.projectRepo.List(req.Keyword, req.Status, offset, req.PageSize)
}

func (s *ProjectService) GetProjectMetrics(req *dto.ProjectMetricsQuery) ([]dto.ProjectMetricVO, error) {
	if req.ProjectID > 0 {
		if _, err := s.projectRepo.GetByID(req.ProjectID); err != nil {
			return nil, errors.New("项目不存在")
		}
	}

	projects, err := s.projectRepo.ListMetricProjects(req.ProjectID, req.IncludeDisabled)
	if err != nil {
		return nil, err
	}

	projectIDs := make([]uint64, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ProjectID)
	}

	countMap, err := s.projectRepo.GetMetricCounts(projectIDs)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ProjectMetricVO, 0, len(projects))
	for _, project := range projects {
		counts := countMap[project.ProjectID]
		rate := 0.0
		if counts.ClosedCount > 0 {
			rate = math.Round(float64(counts.AIResolvedCount)/float64(counts.ClosedCount)*10000) / 100
		}
		items = append(items, dto.ProjectMetricVO{
			ProjectID:          project.ProjectID,
			ProjectName:        project.ProjectName,
			ClosedCount:        counts.ClosedCount,
			AIResolvedCount:    counts.AIResolvedCount,
			AIResolvedRate:     rate,
			ProcessingCount:    counts.ProcessingCount,
			PendingReviewCount: counts.PendingReviewCount,
		})
	}

	return items, nil
}

func (s *ProjectService) ListIssueSyncLogs(projectID uint64, req *dto.ProjectIssueSyncLogQuery) ([]model.IssueSyncLog, int64, error) {
	if _, err := s.projectRepo.GetByID(projectID); err != nil {
		return nil, 0, errors.New("项目不存在")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize

	// 支持按 syncType 筛选
	if req.SyncType != "" {
		return s.syncLogRepo.ListByProjectAndType(projectID, req.SyncType, offset, req.PageSize)
	}
	return s.syncLogRepo.ListByProject(projectID, offset, req.PageSize)
}

func (s *ProjectService) GetIssueSyncLogDetail(projectID, logID uint64) (*model.IssueSyncLog, []model.IssueSyncLogDetail, error) {
	if _, err := s.projectRepo.GetByID(projectID); err != nil {
		return nil, nil, errors.New("项目不存在")
	}

	log, err := s.syncLogRepo.GetByProjectAndID(projectID, logID)
	if err != nil {
		return nil, nil, errors.New("采集记录不存在")
	}

	details, err := s.syncLogRepo.ListDetailsByLogID(logID)
	if err != nil {
		return nil, nil, err
	}

	return log, details, nil
}

// GetIssueSyncLogDetailPaginated 分页获取采集详情
func (s *ProjectService) GetIssueSyncLogDetailPaginated(projectID, logID uint64, page, pageSize int) (*model.IssueSyncLog, []model.IssueSyncLogDetail, int64, error) {
	if _, err := s.projectRepo.GetByID(projectID); err != nil {
		return nil, nil, 0, errors.New("项目不存在")
	}

	log, err := s.syncLogRepo.GetByProjectAndID(projectID, logID)
	if err != nil {
		return nil, nil, 0, errors.New("采集记录不存在")
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	details, total, err := s.syncLogRepo.ListDetailsByLogIDPaginated(logID, offset, pageSize)
	if err != nil {
		return nil, nil, 0, err
	}

	return log, details, total, nil
}

// ListAllIssueSyncLogs 获取所有项目的采集记录
func (s *ProjectService) ListAllIssueSyncLogs(req *dto.IssueSyncLogListQuery) ([]model.IssueSyncLog, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize

	// 支持 syncType 筛选
	var syncType *string
	if req.SyncType != "" {
		syncType = &req.SyncType
	}
	return s.syncLogRepo.ListAll(req.ProjectID, syncType, offset, req.PageSize)
}

// GetIssueSyncLogDetailByID 根据日志ID获取详情（不限项目）
func (s *ProjectService) GetIssueSyncLogDetailByID(logID uint64) (*model.IssueSyncLog, []model.IssueSyncLogDetail, error) {
	log, err := s.syncLogRepo.GetByID(logID)
	if err != nil {
		return nil, nil, errors.New("采集记录不存在")
	}

	details, err := s.syncLogRepo.ListDetailsByLogID(logID)
	if err != nil {
		return nil, nil, err
	}

	return log, details, nil
}

// GetIssueSyncLogDetailByIDPaginated 根据日志ID分页获取详情（不限项目）
func (s *ProjectService) GetIssueSyncLogDetailByIDPaginated(logID uint64, page, pageSize int) (*model.IssueSyncLog, []model.IssueSyncLogDetail, int64, error) {
	log, err := s.syncLogRepo.GetByID(logID)
	if err != nil {
		return nil, nil, 0, errors.New("采集记录不存在")
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	details, total, err := s.syncLogRepo.ListDetailsByLogIDPaginated(logID, offset, pageSize)
	if err != nil {
		return nil, nil, 0, err
	}

	return log, details, total, nil
}
