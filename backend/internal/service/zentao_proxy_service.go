package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

// ZentaoProxyService 禅道 API 代理服务
// 从系统设置中读取禅道连接信息，代理前端请求到禅道 API
type ZentaoProxyService struct {
	settingRepo *repository.SettingRepo
	settingSvc  *SettingService
	logger      *zap.Logger
	httpClient  *http.Client
}

func NewZentaoProxyService(logger *zap.Logger) *ZentaoProxyService {
	return &ZentaoProxyService{
		settingRepo: repository.NewSettingRepo(),
		settingSvc:  NewSettingService(logger),
		logger:      logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// ZentaoProject 禅道项目(简化)
type ZentaoProject struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ZentaoProduct 禅道产品(含 line/type 用于分支关联)
type ZentaoProduct struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`    // normal / branch
	Line    int    `json:"line"`    // 产品线 ID
	Program int    `json:"program"` // 项目集 ID
}

// ZentaoBranch 禅道分支（同产品线下的其他产品）
type ZentaoBranch struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GetProjects 获取禅道项目列表
func (s *ZentaoProxyService) GetProjects() ([]ZentaoProject, error) {
	body, err := s.zentaoGet("/zentao/api.php/v1/projects?limit=200")
	if err != nil {
		return nil, err
	}

	// 禅道返回格式: { "projects": [...] } 或 { "total": N, "projects": [...] }
	var resp struct {
		Projects []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		// 尝试直接解析为数组
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		if err2 := json.Unmarshal(body, &list); err2 != nil {
			return nil, fmt.Errorf("解析项目列表失败: %w, 原始: %s", err, truncate(string(body), 200))
		}
		result := make([]ZentaoProject, len(list))
		for i, p := range list {
			result[i] = ZentaoProject{ID: p.ID, Name: p.Name}
		}
		return result, nil
	}

	result := make([]ZentaoProject, len(resp.Projects))
	for i, p := range resp.Projects {
		result[i] = ZentaoProject{ID: p.ID, Name: p.Name}
	}
	return result, nil
}

// GetProducts 获取禅道产品列表
func (s *ZentaoProxyService) GetProducts() ([]ZentaoProduct, error) {
	body, err := s.zentaoGet("/zentao/api.php/v1/products?limit=200")
	if err != nil {
		return nil, err
	}

	var resp struct {
		Products []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Line    int    `json:"line"`
			Program int    `json:"program"`
		} `json:"products"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		var list []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Line    int    `json:"line"`
			Program int    `json:"program"`
		}
		if err2 := json.Unmarshal(body, &list); err2 != nil {
			return nil, fmt.Errorf("解析产品列表失败: %w, 原始: %s", err, truncate(string(body), 200))
		}
		result := make([]ZentaoProduct, len(list))
		for i, p := range list {
			result[i] = ZentaoProduct{ID: p.ID, Name: p.Name, Type: p.Type, Line: p.Line, Program: p.Program}
		}
		return result, nil
	}

	result := make([]ZentaoProduct, len(resp.Products))
	for i, p := range resp.Products {
		result[i] = ZentaoProduct{ID: p.ID, Name: p.Name, Type: p.Type, Line: p.Line, Program: p.Program}
	}
	return result, nil
}

// GetBranches 获取禅道产品的分支列表
// 禅道 REST API v1 没有独立的 branches 端点，分支数据在产品页面的 HTML 中
// 通过解析 /product-browse-{productID}.html 页面中的 batchChangeBranch 链接提取
func (s *ZentaoProxyService) GetBranches(productID string) ([]ZentaoBranch, error) {
	products, err := s.GetProducts()
	if err == nil {
		if branches := resolveBranchesFromProducts(productID, products); len(branches) > 0 {
			return branches, nil
		}
	}

	baseURL := s.settingRepo.GetValue("zentao", "base_url")
	if baseURL == "" {
		return nil, fmt.Errorf("禅道未配置，请先在「系统设置 → 禅道管理」中配置")
	}

	token, err := s.settingSvc.GetZentaoToken()
	if err != nil {
		return nil, fmt.Errorf("获取禅道Token失败: %w", err)
	}

	base := strings.TrimRight(baseURL, "/")
	url := fmt.Sprintf("%s/product-browse-%s.html", base, productID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求禅道产品页面失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取禅道产品页面失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("禅道返回错误 %d", resp.StatusCode)
	}

	return parseBranchesFromHTML(body), nil
}

func resolveBranchesFromProducts(productID string, products []ZentaoProduct) []ZentaoBranch {
	targetID, err := strconv.Atoi(strings.TrimSpace(productID))
	if err != nil {
		return nil
	}

	var target *ZentaoProduct
	for i := range products {
		if products[i].ID == targetID {
			target = &products[i]
			break
		}
	}
	if target == nil {
		return nil
	}
	if target.Type == "branch" {
		return []ZentaoBranch{{ID: target.ID, Name: strings.TrimSpace(target.Name)}}
	}

	branches := make([]ZentaoBranch, 0)
	seen := make(map[int]struct{})
	appendBranch := func(product ZentaoProduct) {
		if product.ID <= 0 {
			return
		}
		if _, ok := seen[product.ID]; ok {
			return
		}
		seen[product.ID] = struct{}{}
		branches = append(branches, ZentaoBranch{ID: product.ID, Name: strings.TrimSpace(product.Name)})
	}

	for _, product := range products {
		if product.Type != "branch" {
			continue
		}

		sameLine := target.Line > 0 && product.Line == target.Line
		underSelectedProgram := product.Program > 0 && product.Program == target.ID
		if sameLine || underSelectedProgram {
			appendBranch(product)
		}
	}

	return branches
}

func parseBranchesFromHTML(body []byte) []ZentaoBranch {
	// 匹配格式: batchChangeBranch-{branchID}-xxx...>{branchName}</a>
	re := regexp.MustCompile(`batchChangeBranch-(\d+)-[^>]*>([^<]+)</a>`)
	matches := re.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return []ZentaoBranch{}
	}

	branches := make([]ZentaoBranch, 0, len(matches))
	seen := make(map[int]struct{})
	for _, m := range matches {
		id, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		branches = append(branches, ZentaoBranch{ID: id, Name: strings.TrimSpace(m[2])})
	}
	return branches
}

// zentaoGet 发起禅道 GET 请求（自动获取 Token 和 BaseURL）
// 禅道 v4.12 的 API 路径前缀不统一：部分接口用 /api.php/v1/，部分需要 /zentao/api.php/v1/
// 因此自动尝试两种前缀，优先使用有数据的那个
func (s *ZentaoProxyService) zentaoGet(path string) ([]byte, error) {
	baseURL := s.settingRepo.GetValue("zentao", "base_url")
	if baseURL == "" {
		return nil, fmt.Errorf("禅道未配置，请先在「系统设置 → 禅道管理」中配置")
	}

	token, err := s.settingSvc.GetZentaoToken()
	if err != nil {
		return nil, fmt.Errorf("获取禅道Token失败: %w", err)
	}

	base := strings.TrimRight(baseURL, "/")

	// 尝试两种前缀：不带 /zentao 和带 /zentao
	// path 已经以 /zentao/api.php/v1/ 开头，先试原路径，再试去掉 /zentao
	prefixes := []string{path}
	if strings.HasPrefix(path, "/zentao/") {
		prefixes = append(prefixes, strings.TrimPrefix(path, "/zentao"))
	}

	var lastBody []byte
	var lastErr error

	for _, p := range prefixes {
		url := base + p
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Token", token)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("404")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("禅道API返回错误 %d: %s", resp.StatusCode, truncate(string(body), 200))
			continue
		}

		// 如果有内容直接返回
		if len(body) > 0 {
			return body, nil
		}
		lastBody = body
	}

	// 所有前缀都试过了，返回最后一个结果
	if lastBody != nil || lastErr == nil {
		return []byte("[]"), nil // 空响应视为空列表
	}
	return nil, fmt.Errorf("请求禅道API失败: %w", lastErr)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
