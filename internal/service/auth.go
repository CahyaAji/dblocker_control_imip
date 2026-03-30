package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	UserRepo  *repository.UserRepository
	jwtSecret []byte
	apiKey    string // for service-to-service auth (never expires)
}

func NewAuthService(userRepo *repository.UserRepository, jwtSecret string, apiKey string) *AuthService {
	if jwtSecret == "" {
		// Generate a random secret if none provided
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		jwtSecret = hex.EncodeToString(b)
	}
	return &AuthService{
		UserRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
		apiKey:    apiKey,
	}
}

// ValidateAPIKey checks if the provided key matches the configured API key.
func (s *AuthService) ValidateAPIKey(key string) bool {
	return s.apiKey != "" && key == s.apiKey
}

func (s *AuthService) Register(req models.CreateUserRequest) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Username: req.Username,
		Password: string(hash),
		IsAdmin:  req.IsAdmin,
	}

	if err := s.UserRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(username, password string) (string, *models.User, error) {
	user, err := s.UserRepo.FindByUsername(username)
	if err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func (s *AuthService) ValidateToken(tokenStr string) (*models.User, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	user, err := s.UserRepo.FindByID(uint(userID))
	if err != nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

// SeedAdmin creates the default admin user if no users exist.
func (s *AuthService) SeedAdmin(username, password string) error {
	count, err := s.UserRepo.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // users already exist
	}

	_, err = s.Register(models.CreateUserRequest{
		Username: username,
		Password: password,
		IsAdmin:  true,
	})
	return err
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": user.IsAdmin,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 days
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
