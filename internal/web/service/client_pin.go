package service

import (
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// SetPinned changes only the panel presentation preference. It deliberately
// does not touch the Xray client or trigger a core restart.
func (s *ClientService) SetPinned(email string, pinned bool) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("client email is required")
	}
	result := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", email).
		Update("pinned", pinned)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
