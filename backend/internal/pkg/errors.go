package pkg

// 业务错误码定义
const (
	CodeSuccess         = 0
	CodeParamError      = 10001
	CodeUnauthorized    = 10002
	CodeForbidden       = 10003
	CodeNotFound        = 10004
	CodeDuplicate       = 10005
	CodeInternalError   = 10006
	CodeZentaoError     = 20001
	CodeAIError         = 20002
	CodeGitError        = 20003
	CodeMailError       = 20004
	CodeReviewNotFound  = 30001
	CodeReviewNotPending = 30002
)

// 错误消息映射
var codeMessages = map[int]string{
	CodeSuccess:         "成功",
	CodeParamError:      "参数错误",
	CodeUnauthorized:    "未授权",
	CodeForbidden:       "无权限",
	CodeNotFound:        "资源不存在",
	CodeDuplicate:       "资源已存在",
	CodeInternalError:   "内部错误",
	CodeZentaoError:     "禅道接口错误",
	CodeAIError:         "AI服务错误",
	CodeGitError:        "Git操作错误",
	CodeMailError:       "邮件发送错误",
	CodeReviewNotFound:  "Review任务不存在",
	CodeReviewNotPending: "Review任务非待审核状态",
}

func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
