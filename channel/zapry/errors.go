package zapry

import "errors"

var (
	// Profile source construction errors.
	ErrSkillsDirectoryNotFound = errors.New("skills directory not found")
	ErrNoSkillMarkdownFound    = errors.New("no SKILL.md found")

	// Environment config validation errors.
	ErrInvalidAPIBaseURL  = errors.New("invalid API base url")
	ErrInvalidWebhookURL  = errors.New("invalid webhook url")
	ErrWebhookURLRequired = errors.New("webhook url is required for webhook mode")
)
