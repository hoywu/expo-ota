package admin

import (
	"net/http"
	"unicode"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost follows §11.2.
const bcryptCost = 12

var (
	errWeakPassword = httperr.New(http.StatusBadRequest,
		"password must be at least 10 characters and contain both letters and digits")
	errUserNotFound  = httperr.New(http.StatusNotFound, "user not found")
	errUsernameTaken = httperr.New(http.StatusConflict, "username already exists")
	errUsernameEmpty = httperr.New(http.StatusBadRequest, "username must not be empty")
)

// validatePassword enforces the password policy (§11.2): at least 10
// characters with both letters and digits.
func validatePassword(password string) error {
	if len(password) < 10 {
		return errWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return errWeakPassword
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func userToItem(user *models.Users) types.UserItem {
	return types.UserItem{
		Id:          user.Id,
		Username:    user.Username,
		CreatedAt:   formatTime(user.CreatedAt),
		LastLoginAt: formatNullTime(user.LastLoginAt),
		DisabledAt:  formatNullTime(user.DisabledAt),
	}
}
