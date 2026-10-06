package lead

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

type createRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Company string `json:"company"`
	Message string `json:"message"`
}

func (dto *createRequest) normalize() {
	dto.Name = strings.TrimSpace(dto.Name)
	dto.Email = strings.TrimSpace(dto.Email)
	dto.Company = strings.TrimSpace(dto.Company)
	dto.Message = strings.TrimSpace(dto.Message)
}

func (dto createRequest) validate() string {
	switch {
	case dto.Name == "":
		return "name is required"
	case utf8.RuneCountInString(dto.Name) > 200:
		return "name must be at most 200 characters"
	case dto.Email == "":
		return "email is required"
	case len(dto.Email) > 320:
		return "email must be at most 320 characters"
	case utf8.RuneCountInString(dto.Company) > 200:
		return "company must be at most 200 characters"
	case dto.Message == "":
		return "message is required"
	case utf8.RuneCountInString(dto.Message) > 10_000:
		return "message must be at most 10000 characters"
	}
	address, err := mail.ParseAddress(dto.Email)
	if err != nil || address.Address != dto.Email {
		return "email must be valid"
	}
	return ""
}

type response struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Company   string    `json:"company"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func newResponse(lead Lead) response {
	return response{
		ID:        lead.ID,
		Name:      lead.Name,
		Email:     lead.Email,
		Company:   lead.Company,
		Message:   lead.Message,
		CreatedAt: lead.CreatedAt,
	}
}
