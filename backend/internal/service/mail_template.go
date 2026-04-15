package service

import (
	"fmt"
	"strings"
)

const (
	DefaultReviewSubjectTemplate = "[AutoTestFlow] Review结果 - {{title}}"
	DefaultReviewBodyTemplate    = `<h2>Review结果通知</h2>
<p><strong>标题:</strong> {{title}}</p>
<p><strong>状态:</strong> {{status}}</p>
<p><strong>审核意见:</strong> {{review_note}}</p>
<p><strong>Git推送:</strong> {{git_summary}}</p>
<p><strong>问题单:</strong> {{issue_title}}</p>
<p><strong>项目:</strong> {{project_name}}</p>`
	DefaultReportSubjectTemplate = "[AutoTestFlow] 测试报告 - {{title}}"
	DefaultReportBodyTemplate    = `<h2>测试报告: {{title}}</h2>
<p><strong>总用例数:</strong> {{total_cases}}</p>
<p><strong>通过:</strong> {{passed_cases}} | <strong>失败:</strong> {{failed_cases}}</p>
<p><strong>通过率:</strong> {{pass_rate}}%</p>
<p><strong>是否经过人工介入:</strong> {{has_intervention}}</p>
<hr>
<p>{{summary}}</p>
<p><a href="{{report_url}}">查看完整报告</a></p>`
)

func renderMailTemplate(tpl string, values map[string]string) string {
	if strings.TrimSpace(tpl) == "" {
		return ""
	}

	result := tpl
	for key, value := range values {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

func defaultMailTemplate(key string) string {
	switch key {
	case "review_result_subject_template":
		return DefaultReviewSubjectTemplate
	case "review_result_body_template":
		return DefaultReviewBodyTemplate
	case "test_report_subject_template":
		return DefaultReportSubjectTemplate
	case "test_report_body_template":
		return DefaultReportBodyTemplate
	default:
		return ""
	}
}

func buildSampleTemplateData(templateType string) map[string]string {
	switch templateType {
	case "review_result":
		return map[string]string{
			"title":        "Review: 用户登录后 token 未正确刷新",
			"status":       "approved",
			"review_note":  "这是一封用于验证模板渲染的测试邮件。",
			"git_summary":  "已推送到Git仓库",
			"issue_title":  "用户登录后 token 未正确刷新",
			"project_name": "CRM-A7",
			"review_id":    "1001",
		}
	case "test_report":
		return map[string]string{
			"title":            "CRM-A7 每日回归测试",
			"summary":          "本次共执行 20 条用例，其中 18 条通过，2 条失败。",
			"total_cases":      "20",
			"passed_cases":     "18",
			"failed_cases":     "2",
			"pass_rate":        "90.0",
			"has_intervention": "否",
			"report_url":       "http://localhost:3000/executions",
		}
	default:
		return map[string]string{
			"title": "AutoTestFlow 测试邮件",
		}
	}
}

func templateDescription() string {
	return "可用变量：{{title}} {{status}} {{review_note}} {{git_summary}} {{issue_title}} {{project_name}} {{summary}} {{total_cases}} {{passed_cases}} {{failed_cases}} {{pass_rate}} {{has_intervention}} {{report_url}}"
}

func formatPassRate(passRate float64) string {
	return fmt.Sprintf("%.1f", passRate)
}
