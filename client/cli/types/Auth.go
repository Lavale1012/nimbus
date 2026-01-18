package types

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
	Email   string `json:"email"`
	UserID  uint   `json:"user_id"`
	Box     string `json:"box"`
}
