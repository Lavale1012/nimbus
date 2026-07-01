package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/handlers/user"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// resetRouter wires just the reset-password handler for isolated handler tests.
// Rate limiting is exercised separately in the ratelimit package tests.
func resetRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/reset-password", func(c *gin.Context) {
		user.ResetPassword(c, db)
	})
	return r
}

// doReset is a small helper to POST a reset request and return the recorder.
func doReset(r *gin.Engine, payload map[string]string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "/reset-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestResetPassword_Success(t *testing.T) {
	db := setupLoginDB(t)
	// seedLoginUser sets the passkey to "1234".
	seedLoginUser(t, db, "reset@example.com", "OldPass123!@#")
	r := resetRouter(db)

	w := doReset(r, map[string]string{
		"email":        "reset@example.com",
		"passkey":      "1234",
		"new_password": "NewPass456!@#",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Password reset successful", resp["message"])

	// The new password hash must now be stored, and the old one must not verify.
	var updated models.User
	db.Where("email = ?", "reset@example.com").First(&updated)
	assert.True(t, utils.VerifyPasswordHash("NewPass456!@#", updated.Password))
	assert.False(t, utils.VerifyPasswordHash("OldPass123!@#", updated.Password))
}

func TestResetPassword_WrongPasskey(t *testing.T) {
	db := setupLoginDB(t)
	seedLoginUser(t, db, "reset2@example.com", "OldPass123!@#")
	r := resetRouter(db)

	w := doReset(r, map[string]string{
		"email":        "reset2@example.com",
		"passkey":      "9999",
		"new_password": "NewPass456!@#",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Invalid email or passkey", resp["error"])

	// Password must be unchanged.
	var unchanged models.User
	db.Where("email = ?", "reset2@example.com").First(&unchanged)
	assert.True(t, utils.VerifyPasswordHash("OldPass123!@#", unchanged.Password))
}

func TestResetPassword_UnknownEmail(t *testing.T) {
	db := setupLoginDB(t)
	r := resetRouter(db)

	// Same generic error as a wrong passkey — no email enumeration.
	w := doReset(r, map[string]string{
		"email":        "ghost@example.com",
		"passkey":      "1234",
		"new_password": "NewPass456!@#",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Invalid email or passkey", resp["error"])
}

func TestResetPassword_WeakNewPassword(t *testing.T) {
	db := setupLoginDB(t)
	seedLoginUser(t, db, "reset3@example.com", "OldPass123!@#")
	r := resetRouter(db)

	// Missing uppercase, number, and symbol.
	w := doReset(r, map[string]string{
		"email":        "reset3@example.com",
		"passkey":      "1234",
		"new_password": "weakpassword",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResetPassword_SameAsOld(t *testing.T) {
	db := setupLoginDB(t)
	seedLoginUser(t, db, "reset4@example.com", "OldPass123!@#")
	r := resetRouter(db)

	w := doReset(r, map[string]string{
		"email":        "reset4@example.com",
		"passkey":      "1234",
		"new_password": "OldPass123!@#",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "New password must be different from the current password", resp["error"])
}

func TestResetPassword_MissingFields(t *testing.T) {
	db := setupLoginDB(t)
	r := resetRouter(db)

	cases := []map[string]string{
		{"passkey": "1234", "new_password": "NewPass456!@#"},        // no email
		{"email": "x@example.com", "new_password": "NewPass456!@#"}, // no passkey
		{"email": "x@example.com", "passkey": "1234"},               // no new password
		{}, // nothing
	}
	for _, payload := range cases {
		w := doReset(r, payload)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestResetPassword_WrongPasskeyLength(t *testing.T) {
	db := setupLoginDB(t)
	seedLoginUser(t, db, "reset5@example.com", "OldPass123!@#")
	r := resetRouter(db)

	w := doReset(r, map[string]string{
		"email":        "reset5@example.com",
		"passkey":      "12345", // 5 chars, must be exactly 4
		"new_password": "NewPass456!@#",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
