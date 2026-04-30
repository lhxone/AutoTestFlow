package model

import "time"

const (
	APIExchangeDirectionInbound  = "inbound"
	APIExchangeDirectionOutbound = "outbound"

	APIExchangeStatusSuccess = "success"
	APIExchangeStatusFailed  = "failed"
)

// APIExchangeLog records request/response history for public integration APIs
// and outbound calls to external systems.
type APIExchangeLog struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	APIName          string    `gorm:"size:64;index;not null" json:"api_name"`
	Direction        string    `gorm:"size:16;index;not null" json:"direction"`
	Method           string    `gorm:"size:16;not null" json:"method"`
	URL              string    `gorm:"size:512;not null;default:''" json:"url"`
	RemoteAddr       string    `gorm:"size:128;not null;default:''" json:"remote_addr"`
	RequestHeaders   JSON      `gorm:"type:json" json:"request_headers"`
	RequestBody      string    `gorm:"type:longtext" json:"request_body"`
	ResponseStatus   int       `gorm:"not null;default:0" json:"response_status"`
	ResponseBody     string    `gorm:"type:longtext" json:"response_body"`
	ResultStatus     string    `gorm:"size:16;index;not null;default:'success'" json:"result_status"`
	ErrorMessage     string    `gorm:"type:text" json:"error_message"`
	DurationMillis   int64     `gorm:"not null;default:0" json:"duration_millis"`
	RelatedIssueID   *uint64   `gorm:"index" json:"related_issue_id"`
	RelatedTaskID    *uint64   `gorm:"index" json:"related_task_id"`
	RelatedDevTaskID string    `gorm:"size:191;index;not null;default:''" json:"related_dev_task_id"`
	CreatedAt        time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (APIExchangeLog) TableName() string { return "api_exchange_log" }
