package tool

import (
	"crypto/md5"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

func Md5ByString(str string) (string, error) {
	m := md5.New()
	_, err := io.WriteString(m, str)
	if err != nil {
		return "", err
	}
	arr := m.Sum(nil)
	return fmt.Sprintf("%x", arr), nil
}

func Md5ByBytes(b []byte) string {
	return fmt.Sprintf("%x", md5.Sum(b))
}

const bcryptCost = 10

// HashPassword returns the bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}

// CheckPassword compares a bcrypt hashed password with a plaintext candidate.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
