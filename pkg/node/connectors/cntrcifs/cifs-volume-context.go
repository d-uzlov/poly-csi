package cntrcifs

import (
	"fmt"
)

type CifsMountContext struct {
	ShareName    string
	UserName     string
	UserPassword string
}

const (
	shareNameKey = "cifs.share_name"
	userNameKey  = "cifs.user_name"
	userPassKey  = "cifs.user_pass"
)

func (c CifsMountContext) InjectPublic(dst map[string]string) error {
	dst[shareNameKey] = c.ShareName
	dst[userNameKey] = c.UserName
	return nil
}

func (c CifsMountContext) InjectSecret(dst map[string]string) error {
	dst[userPassKey] = c.UserPassword
	return nil
}

func ParsePublishContext(public map[string]string, secret map[string]string) (CifsMountContext, error) {
	shareName, ok := public[shareNameKey]
	if !ok {
		return CifsMountContext{}, fmt.Errorf("missing field %s", shareNameKey)
	}
	userName, ok := public[userNameKey]
	if !ok {
		return CifsMountContext{}, fmt.Errorf("missing field %s", userNameKey)
	}
	userPassword, ok := secret[userPassKey]
	if !ok {
		return CifsMountContext{}, fmt.Errorf("missing field %s", userPassKey)
	}
	return CifsMountContext{
		ShareName:    shareName,
		UserName:     userName,
		UserPassword: userPassword,
	}, nil
}
