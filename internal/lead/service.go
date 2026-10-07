package lead

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
)

type Service struct {
	classifier Classifier
	repository Repository
}

func NewService(classifier Classifier, repository Repository) *Service {
	return &Service{classifier: classifier, repository: repository}
}

type ValidationError string

func (err ValidationError) Error() string { return string(err) }

func (s *Service) Create(ctx context.Context, params CreateParams) (Lead, error) {
	params.normalize()
	if message := params.validate(); message != "" {
		return Lead{}, ValidationError(message)
	}

	lead := Lead{
		Name:    params.Name,
		Email:   params.Email,
		Company: params.Company,
		Message: params.Message,
	}
	classification, err := s.classifier.Classify(ctx, lead)
	if err != nil {
		return Lead{}, fmt.Errorf("classify lead: %w", err)
	}
	lead.Classification = classification

	created, err := s.repository.Create(ctx, lead)
	if err != nil {
		return Lead{}, fmt.Errorf("persist lead: %w", err)
	}
	return created, nil
}

func (params *CreateParams) normalize() {
	params.Name = strings.TrimSpace(params.Name)
	params.Email = strings.TrimSpace(params.Email)
	params.Company = strings.TrimSpace(params.Company)
	params.Message = strings.TrimSpace(params.Message)
}

func (params CreateParams) validate() string {
	switch {
	case params.Name == "":
		return "name is required"
	case utf8.RuneCountInString(params.Name) > 200:
		return "name must be at most 200 characters"
	case params.Email == "":
		return "email is required"
	case len(params.Email) > 320:
		return "email must be at most 320 characters"
	case utf8.RuneCountInString(params.Company) > 200:
		return "company must be at most 200 characters"
	case params.Message == "":
		return "message is required"
	case utf8.RuneCountInString(params.Message) > 10_000:
		return "message must be at most 10000 characters"
	}
	address, err := mail.ParseAddress(params.Email)
	if err != nil || address.Address != params.Email {
		return "email must be valid"
	}
	return ""
}
