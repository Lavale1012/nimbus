// Package user contains HTTP handlers for user registration and login.
package user

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

// ── Constants ────────────────────────────────────────────────────────────────

// MAX_PASSWORD_LENGTH is 72 because bcrypt silently truncates passwords longer
// than 72 bytes — we reject them upfront so users aren't surprised.
const (
	MAX_EMAIL_LENGTH    = 254
	MAX_PASSWORD_LENGTH = 72
	MIN_PASSWORD_LENGTH = 8
	PASSKEY_LENGTH      = 4

	dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
)

// ── Types ────────────────────────────────────────────────────────────────────

// LoginRequest is the JSON body expected by the /login endpoint.
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest is the JSON body expected by the /register endpoint.
// Using a dedicated struct keeps sensitive fields out of the User model's JSON tags.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	PassKey  string `json:"passkey" binding:"required"`
}

// ── HTML ─────────────────────────────────────────────────────────────────────

const registerPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>Nimbus — Register</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0f1117; color: #e0e0e0;
      display: flex; justify-content: center; align-items: center; min-height: 100vh;
    }
    .card {
      background: #1a1d27; border: 1px solid #2e3148;
      border-radius: 10px; padding: 40px; width: 420px;
    }
    h1 { font-size: 1.5rem; margin-bottom: 4px; color: #5b9bd5; }
    p.sub { font-size: 0.85rem; color: #666; margin-bottom: 28px; }
    label { display: block; font-size: 0.8rem; color: #4ec9b0; margin-bottom: 6px; }
    input {
      width: 100%; padding: 10px 12px; background: #12141e;
      border: 1px solid #2e3148; border-radius: 6px;
      color: #e0e0e0; font-size: 0.95rem; margin-bottom: 18px; outline: none;
    }
    input:focus { border-color: #5b9bd5; }
    button {
      width: 100%; padding: 11px; background: #5b9bd5; color: #fff;
      border: none; border-radius: 6px; font-size: 1rem; cursor: pointer; margin-top: 4px;
    }
    button:hover { background: #4a87c0; }
    button:disabled { background: #333; cursor: default; }
    .msg { margin-top: 16px; font-size: 0.9rem; text-align: center; min-height: 22px; }
    .msg.err { color: #ff6b6b; }
    .msg.ok  { color: #6bcb77; }
  </style>
</head>
<body>
  <div class="card">
    <h1>☁ Nimbus CLI</h1>
    <p class="sub">Create a new account</p>
    <form id="form">
      <label>Email</label>
      <input type="email" name="email" placeholder="you@example.com" required />
      <label>Password</label>
      <input type="password" name="password" placeholder="Min 8 chars, upper, lower, number, symbol" required />
      <label>Confirm Password</label>
      <input type="password" name="confirm" placeholder="Repeat password" required />
      <label>Passkey <span style="color:#666">(exactly 4 characters)</span></label>
      <input type="text" name="passkey" maxlength="4" placeholder="e.g. 1234" required />
      <button type="submit">Create Account</button>
      <div class="msg" id="msg"></div>
    </form>
  </div>
  <script>
    document.getElementById("form").addEventListener("submit", async e => {
      e.preventDefault();
      const msg = document.getElementById("msg");
      const btn = document.querySelector("button");
      const data = Object.fromEntries(new FormData(e.target));
      if (data.password !== data.confirm) {
        msg.className = "msg err"; msg.textContent = "Passwords do not match."; return;
      }
      if (data.passkey.length !== 4) {
        msg.className = "msg err"; msg.textContent = "Passkey must be exactly 4 characters."; return;
      }
      msg.className = "msg"; msg.textContent = "Creating account...";
      btn.disabled = true;
      try {
        const res = await fetch("/v1/api/auth/users/register", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email: data.email, password: data.password, passkey: data.passkey })
        });
        const json = await res.json();
        if (!res.ok) {
          msg.className = "msg err";
          msg.textContent = json.error || "Registration failed.";
          btn.disabled = false;
        } else {
          msg.className = "msg ok";
          msg.textContent = "Account created! You can close this tab and run: nim login";
        }
      } catch {
        msg.className = "msg err";
        msg.textContent = "Could not reach the server.";
        btn.disabled = false;
      }
    });
  </script>
</body>
</html>`

// ── Helpers ──────────────────────────────────────────────────────────────────

// isValidPassword checks complexity: min length, number, upper, lower, special.
func isValidPassword(s string) (minLength, number, upper, lower, special bool) {
	var hasNumber, hasUpper, hasLower, hasSpecial bool
	for _, c := range s {
		switch {
		case unicode.IsNumber(c):
			hasNumber = true
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}
	minLength = len(s) >= MIN_PASSWORD_LENGTH
	number = hasNumber
	upper = hasUpper
	lower = hasLower
	special = hasSpecial
	return
}

// isEmailValid uses a regex to check basic email format.
func isEmailValid(e string) bool {
	emailRegex := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return emailRegex.MatchString(e)
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// ServeRegisterPage returns the HTML registration form.
func ServeRegisterPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, registerPage)
}

// Login validates credentials and returns a signed JWT on success.
// If the email doesn't exist we still run bcrypt on a dummy hash so the
// response time is the same as a real password mismatch — prevents email enumeration.
func Login(c *gin.Context, db *gorm.DB) {
	var user models.User
	var loginRequest LoginRequest

	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if loginRequest.Email == "" || loginRequest.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	if len(loginRequest.Email) > MAX_EMAIL_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if len(loginRequest.Password) > MAX_PASSWORD_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if !isEmailValid(loginRequest.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	err := db.Preload("Boxes").Where("email = ?", loginRequest.Email).First(&user).Error
	var isValid bool

	if err != nil {
		utils.VerifyPasswordHash(loginRequest.Password, dummyHash)
		isValid = false
	} else {
		isValid = utils.VerifyPasswordHash(loginRequest.Password, user.Password)
	}

	if !isValid {
		log.Printf("Failed login attempt for email: %s from IP: %s", loginRequest.Email, c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := jwt.CreateToken(user.Email, fmt.Sprintf("%d", user.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "token": token, "user_id": user.ID, "email": user.Email, "box": user.Boxes})
}

// Register creates a new user account:
//  1. Validate and sanitize all input fields
//  2. Check for duplicate email
//  3. Hash the password and passkey with bcrypt
//  4. Generate a random 8-digit user ID (retrying on the rare collision)
//  5. Create the user record along with their default "Home-Box"
func Register(c *gin.Context, db *gorm.DB, s3Client *s3.Client) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if req.Email == "" || req.Password == "" || req.PassKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, password, and passkey are required"})
		return
	}

	if !isEmailValid(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	if len(req.Email) > MAX_EMAIL_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email exceeds maximum allowed length"})
		return
	}

	if len(req.Password) < MIN_PASSWORD_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters long"})
		return
	}

	if len(req.Password) > MAX_PASSWORD_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password exceeds maximum allowed length"})
		return
	}

	if len(req.PassKey) != PASSKEY_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passkey must be exactly 4 characters long"})
		return
	}

	minLength, number, upper, lower, special := isValidPassword(req.Password)
	if !minLength || !number || !upper || !lower || !special {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password must be at least 8 characters and include at least one number, one uppercase letter, one lowercase letter, and one special character",
		})
		return
	}

	var existingUser models.User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	hashedPassword, err := utils.PasswordHash(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	hashedPassKey, err := utils.PasswordHash(req.PassKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	var user models.User
	user.Email = req.Email
	user.Password = hashedPassword
	user.PassKey = hashedPassKey

	userID, err := utils.GenerateUserID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	var existingUserByID models.User
	for {
		if err := db.First(&existingUserByID, userID).Error; err != nil {
			break
		}
		userID, err = utils.GenerateUserID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
			return
		}
	}
	user.ID = userID

	boxID, err := utils.GenerateSecureID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	user.Boxes = []models.Box{{
		Name:  "Home-Box",
		BoxID: boxID,
	}}

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user bucket"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"email":   user.Email,
		"user_id": user.ID,
		"box":     user.Boxes[0].Name,
	})
}
