package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Config 保存服务运行参数。
type Config struct {
	MainListen string
	HTTPListen string

	IdleTimeout time.Duration
	Accounts    []Account
}

// Account 表示允许接入的下级平台注册信息。
type Account struct {
	UserID     uint32
	Password   string
	VerifyCode uint32
}

// normalizeHostPort 将 host:port 字符串拆分为 host 与 port，便于 go-server 初始化。
func normalizeHostPort(addr string) (string, int, error) {
	if addr == "" {
		return "", 0, errors.New("address must not be empty")
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("split host/port %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("parse port %q: %w", portStr, err)
	}
	return host, port, nil
}

// MultiAccountFlag 支持重复声明账号参数。
type MultiAccountFlag []Account

func (m *MultiAccountFlag) String() string {
	parts := make([]string, 0, len(*m))
	for _, acc := range *m {
		parts = append(parts, fmt.Sprintf("%d:%s:%d", acc.UserID, acc.Password, acc.VerifyCode))
	}
	return strings.Join(parts, ",")
}

func (m *MultiAccountFlag) Set(value string) error {
	parts := strings.Split(value, ":")
	if len(parts) < 3 || len(parts) > 4 {
		return errors.New("account must be formatted as userID:password:verifyCode")
	}
	userID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return fmt.Errorf("parse user id %q: %w", parts[0], err)
	}
	verify, err := strconv.ParseUint(parts[2], 0, 32)
	if err != nil {
		return fmt.Errorf("parse verify code %q: %w", parts[2], err)
	}
	acc := Account{
		UserID:     uint32(userID),
		Password:   parts[1],
		VerifyCode: uint32(verify),
	}
	if len(parts) == 4 {
		if parts[3] != "2019" {
			return fmt.Errorf("unsupported version %q, only 2019 is available", parts[3])
		}
	}
	*m = append(*m, acc)
	return nil
}
