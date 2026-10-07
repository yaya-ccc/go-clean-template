package user

import (
	"context"
	"testing"

	"go-clean-template/backend/internal/application/user/dto"
	"go-clean-template/backend/internal/domain/user/fakes"
	"go-clean-template/backend/internal/kernel/apperr"
)

func TestGetMeOwnsMissingUserSemantics(t *testing.T) {
	repo := fakes.NewUsers()
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetMe(context.Background(), dto.GetMeInput{UserID: 999}); !apperr.CodeIs(err, apperr.ErrUnauthorized) {
		t.Fatalf("已认证用户消失时用例应直接要求重登，得到 %v", err)
	}
	u, err := repo.Create(context.Background(), "a@example.com", "hash", "a")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetMe(context.Background(), dto.GetMeInput{UserID: u.ID})
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("应返回身份对应用户，得到 %v %v", got, err)
	}
}
