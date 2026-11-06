package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 19 * 1024 // KiB (19 MiB)
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

type argon2PasswordService struct{}

func NewArgon2PasswordService() PasswordService {
	return argon2PasswordService{}
}

func (argon2PasswordService) Hash(_ context.Context, password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads, encode(salt), encode(hash)), nil
}

func (argon2PasswordService) Verify(_ context.Context, password, encodedHash string) (bool, error) {
	if password == "" {
		return false, nil
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid password hash format")
	}

	var memory, iterations uint32
	var threads uint8
	for _, parameter := range strings.Split(parts[3], ",") {
		keyValue := strings.SplitN(parameter, "=", 2)
		if len(keyValue) != 2 {
			return false, errors.New("invalid password hash parameters")
		}
		value, err := strconv.ParseUint(keyValue[1], 10, 32)
		if err != nil {
			return false, errors.New("invalid password hash parameters")
		}
		switch keyValue[0] {
		case "m":
			memory = uint32(value)
		case "t":
			iterations = uint32(value)
		case "p":
			if value > 255 {
				return false, errors.New("invalid password hash parameters")
			}
			threads = uint8(value)
		default:
			return false, errors.New("invalid password hash parameters")
		}
	}
	// Accept only our configured parameters to prevent attacker-controlled resource use.
	if memory != argonMemory || iterations != argonTime || threads != argonThreads {
		return false, errors.New("unsupported password hash parameters")
	}
	decode := base64.RawStdEncoding.DecodeString
	salt, err := decode(parts[4])
	if err != nil || len(salt) != argonSaltLen {
		return false, errors.New("invalid password hash salt")
	}
	want, err := decode(parts[5])
	if err != nil || len(want) != int(argonKeyLen) {
		return false, errors.New("invalid password hash digest")
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
