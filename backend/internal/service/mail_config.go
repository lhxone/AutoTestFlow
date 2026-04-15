package service

import (
	"strconv"
	"strings"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/repository"
)

type MailConfig struct {
	Host              string
	Port              int
	Username          string
	Password          string
	From              string
	UseSSL            bool
	DefaultRecipients []string
}

func LoadMailConfig() MailConfig {
	repo := repository.NewSettingRepo()
	cfg := MailConfig{
		Host:     firstMailNonEmpty(repo.GetValue("mail", "host"), config.Global.Mail.Host),
		Port:     firstMailPositiveInt(repo.GetValue("mail", "port"), config.Global.Mail.Port),
		Username: firstMailNonEmpty(repo.GetValue("mail", "username"), config.Global.Mail.Username),
		Password: firstMailNonEmpty(repo.GetValue("mail", "password"), config.Global.Mail.Password),
		From:     firstMailNonEmpty(repo.GetValue("mail", "from"), config.Global.Mail.From),
		UseSSL:   firstMailBool(repo.GetValue("mail", "use_ssl"), config.Global.Mail.UseSSL),
	}

	cfg.DefaultRecipients = splitRecipients(repo.GetValue("mail", "default_recipients"))
	return cfg
}

func splitRecipients(raw string) []string {
	if raw == "" {
		return nil
	}
	items := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, value)
	}
	return result
}

func firstMailNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstMailPositiveInt(value string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func firstMailBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
