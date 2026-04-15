package dto

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=2,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"` // 秒
	User         UserInfo `json:"user"`
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserInfo 用户信息(响应用)
type UserInfo struct {
	ID       uint64   `json:"id"`
	Username string   `json:"username"`
	RealName string   `json:"real_name"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Avatar   string   `json:"avatar"`
	Status   int8     `json:"status"`
	Roles    []string `json:"roles"`
	Permissions []string `json:"permissions"`
}
