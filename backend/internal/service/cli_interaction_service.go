package service

import (
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
	"errors"

	"go.uber.org/zap"
)

type CLIInteractionService struct {
	logger *zap.Logger
	repo   *repository.CLIInteractionRepo
}

func NewCLIInteractionService(logger *zap.Logger) *CLIInteractionService {
	return &CLIInteractionService{
		logger: logger,
		repo:   repository.NewCLIInteractionRepo(),
	}
}

func (s *CLIInteractionService) GetByTaskID(taskID uint) ([]model.CLIInteraction, error) {
	return s.repo.GetByTaskID(taskID)
}

func (s *CLIInteractionService) GetPendingByTaskID(taskID uint) ([]model.CLIInteraction, error) {
	return s.repo.GetPendingByTaskID(taskID)
}

func (s *CLIInteractionService) ReplyInteraction(id, userID uint, response string) error {
	interaction, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("交互记录不存在")
	}

	if interaction.Status != "pending" {
		return errors.New("该交互已处理")
	}

	return s.repo.UpdateResponse(id, userID, "answered", response)
}

func (s *CLIInteractionService) ApproveInteraction(id, userID uint) error {
	interaction, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("交互记录不存在")
	}

	if interaction.Status != "pending" {
		return errors.New("该交互已处理")
	}

	return s.repo.UpdateResponse(id, userID, "approved", "approved")
}

func (s *CLIInteractionService) RejectInteraction(id, userID uint, reason string) error {
	interaction, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("交互记录不存在")
	}

	if interaction.Status != "pending" {
		return errors.New("该交互已处理")
	}

	return s.repo.UpdateResponse(id, userID, "rejected", reason)
}
