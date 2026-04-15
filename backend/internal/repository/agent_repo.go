package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type AgentRepo struct {
	db *gorm.DB
}

func NewAgentRepo() *AgentRepo {
	return &AgentRepo{db: DB}
}

func (r *AgentRepo) Create(agent *model.Agent) error {
	return r.db.Create(agent).Error
}

func (r *AgentRepo) GetByID(id uint64) (*model.Agent, error) {
	var agent model.Agent
	err := r.db.Preload("Skills").Preload("MCPServers").First(&agent, id).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *AgentRepo) Update(agent *model.Agent) error {
	return r.db.Omit("Skills", "MCPServers").Save(agent).Error
}

func (r *AgentRepo) Delete(id uint64) error {
	return r.db.Delete(&model.Agent{}, id).Error
}

func (r *AgentRepo) List(keyword string, status *int8, offset, limit int) ([]model.Agent, int64, error) {
	query := r.db.Model(&model.Agent{}).Preload("Skills").Preload("MCPServers")
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var agents []model.Agent
	if err := query.Offset(offset).Limit(limit).Order("is_default DESC, id DESC").Find(&agents).Error; err != nil {
		return nil, 0, err
	}
	return agents, total, nil
}

// GetDefaultActive 获取默认且启用的Agent
func (r *AgentRepo) GetDefaultActive() (*model.Agent, error) {
	var agent model.Agent
	err := r.db.Preload("Skills").Where("status = 1 AND is_default = 1").Order("id ASC").First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetFirstActive 获取第一个启用的Agent
func (r *AgentRepo) GetFirstActive() (*model.Agent, error) {
	var agent model.Agent
	err := r.db.Preload("Skills").Where("status = 1").Order("id ASC").First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// SetDefault 将指定 Agent 设置为默认，并清除其他 Agent 的默认标记。
func (r *AgentRepo) SetDefault(agentID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Agent{}).Where("is_default = 1").Update("is_default", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Agent{}).Where("id = ?", agentID).Update("is_default", true).Error; err != nil {
			return err
		}
		return nil
	})
}

// SetSkills 设置Agent的Skill绑定
func (r *AgentRepo) SetSkills(agentID uint64, skillIDs []uint64) error {
	if err := r.db.Where("agent_id = ?", agentID).Delete(&model.AgentSkill{}).Error; err != nil {
		return err
	}
	for _, sid := range skillIDs {
		as := model.AgentSkill{AgentID: agentID, SkillID: sid}
		if err := r.db.Create(&as).Error; err != nil {
			return err
		}
	}
	return nil
}

// SetMCPServers 设置Agent的MCP绑定
func (r *AgentRepo) SetMCPServers(agentID uint64, mcpIDs []uint64) error {
	if err := r.db.Where("agent_id = ?", agentID).Delete(&model.AgentMCP{}).Error; err != nil {
		return err
	}
	for _, mid := range mcpIDs {
		am := model.AgentMCP{AgentID: agentID, MCPServerID: mid}
		if err := r.db.Create(&am).Error; err != nil {
			return err
		}
	}
	return nil
}

// Skill CRUD
type SkillRepo struct {
	db *gorm.DB
}

func NewSkillRepo() *SkillRepo {
	return &SkillRepo{db: DB}
}

func (r *SkillRepo) Create(skill *model.Skill) error  { return r.db.Create(skill).Error }
func (r *SkillRepo) Update(skill *model.Skill) error  { return r.db.Save(skill).Error }
func (r *SkillRepo) Delete(id uint64) error            { return r.db.Delete(&model.Skill{}, id).Error }

func (r *SkillRepo) GetByID(id uint64) (*model.Skill, error) {
	var s model.Skill
	return &s, r.db.First(&s, id).Error
}

func (r *SkillRepo) GetByName(name string) (*model.Skill, error) {
	var s model.Skill
	return &s, r.db.Where("name = ?", name).First(&s).Error
}

func (r *SkillRepo) ListAll() ([]model.Skill, error) {
	var skills []model.Skill
	return skills, r.db.Order("id ASC").Find(&skills).Error
}

// MCPServer CRUD
type MCPServerRepo struct {
	db *gorm.DB
}

func NewMCPServerRepo() *MCPServerRepo {
	return &MCPServerRepo{db: DB}
}

func (r *MCPServerRepo) Create(s *model.MCPServer) error  { return r.db.Create(s).Error }
func (r *MCPServerRepo) Update(s *model.MCPServer) error  { return r.db.Save(s).Error }
func (r *MCPServerRepo) Delete(id uint64) error            { return r.db.Delete(&model.MCPServer{}, id).Error }

func (r *MCPServerRepo) GetByID(id uint64) (*model.MCPServer, error) {
	var s model.MCPServer
	return &s, r.db.First(&s, id).Error
}

func (r *MCPServerRepo) ListAll() ([]model.MCPServer, error) {
	var servers []model.MCPServer
	return servers, r.db.Order("id ASC").Find(&servers).Error
}
