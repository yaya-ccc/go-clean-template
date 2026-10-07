package auth

import (
	authdto "go-clean-template/backend/internal/application/auth/dto"
	"go-clean-template/backend/internal/presentation/api/auth/dto"
)

func toRegisterInput(req *dto.RegisterRequest) authdto.RegisterInput {
	return authdto.RegisterInput{Email: req.Email, Password: req.Password, Nickname: req.Nickname}
}

func toLoginInput(req *dto.LoginRequest) authdto.LoginInput {
	return authdto.LoginInput{Email: req.Email, Password: req.Password}
}

func toRefreshInput(raw string) authdto.RefreshInput {
	return authdto.RefreshInput{RefreshToken: raw}
}

func toLogoutInput(raw string) authdto.LogoutInput {
	return authdto.LogoutInput{RefreshToken: raw}
}
