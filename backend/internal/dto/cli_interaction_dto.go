package dto

type CLIInteractionResponse struct {
	ID              uint   `json:"id"`
	TaskID          uint   `json:"task_id"`
	InteractionType string `json:"interaction_type"`
	Content         string `json:"content"`
	Metadata        any    `json:"metadata"`
	Status          string `json:"status"`
	UserResponse    string `json:"user_response"`
	UserID          uint   `json:"user_id"`
	RespondedAt     string `json:"responded_at"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type ReplyInteractionRequest struct {
	Response string `json:"response" binding:"required"`
}

type ApprovalRequest struct {
	Reason string `json:"reason"`
}
