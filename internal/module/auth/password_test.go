package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArgon2PasswordService(t *testing.T) {
	service := NewArgon2PasswordService()
	ctx := context.Background()

	hash, err := service.Hash(ctx, "correct horse battery staple")
	require.NoError(t, err)
	require.Contains(t, hash, "$argon2id$v=19$")

	valid, err := service.Verify(ctx, "correct horse battery staple", hash)
	require.NoError(t, err)
	require.True(t, valid)

	valid, err = service.Verify(ctx, "wrong password", hash)
	require.NoError(t, err)
	require.False(t, valid)
}
