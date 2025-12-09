package server

import (
	"math/rand"
	"sync"
	"time"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

// Authenticator 基于静态账号表实现登录校验。
type Authenticator struct {
	mu       sync.RWMutex
	accounts map[uint32]Account
}

func NewAuthenticator(accounts []Account) *Authenticator {
	m := make(map[uint32]Account, len(accounts))
	for _, acc := range accounts {
		m[acc.UserID] = acc
	}
	return &Authenticator{accounts: m}
}

// Authenticate 校验账号密码，返回账号信息与登录应答。
func (a *Authenticator) Authenticate(req jtt809.LoginRequest, clientIP string) (Account, jtt809.LoginResponse) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	acc, ok := a.accounts[req.UserID]
	if !ok {
		return Account{}, jtt809.LoginResponse{Result: jtt809.LoginUnregistered}
	}
	resp := jtt809.LoginResponse{
		Result: jtt809.LoginOK,
	}
	if !isIPAllowed(clientIP, acc.AllowIPs) {
		resp.Result = jtt809.LoginIPError
		return acc, resp
	}
	if req.GnssCenterID != acc.GnssCenterID {
		resp.Result = jtt809.LoginGnssCenterIDError
		return acc, resp
	}
	if req.Password != acc.Password {
		resp.Result = jtt809.LoginPasswordError
		return acc, resp
	}
	// 随机生成一个uint32校验码
	resp.VerifyCode = rand.New(rand.NewSource(time.Now().UnixNano())).Uint32()
	return acc, resp
}

// Lookup 返回账号信息。
func (a *Authenticator) Lookup(userID uint32) (Account, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	acc, ok := a.accounts[userID]
	return acc, ok
}

// AddAccounts 批量新增或更新账号，返回被覆盖的用户ID列表。
func (a *Authenticator) AddAccounts(accs []Account) []uint32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	replaced := make([]uint32, 0, len(accs))
	for _, acc := range accs {
		if _, ok := a.accounts[acc.UserID]; ok {
			replaced = append(replaced, acc.UserID)
		}
		a.accounts[acc.UserID] = acc
	}
	return replaced
}

// AddAccount 新增或更新账号。
// 返回值表示是否覆盖了已有账号。
func (a *Authenticator) AddAccount(acc Account) bool {
	replaced := a.AddAccounts([]Account{acc})
	return len(replaced) > 0
}

// RemoveAccount 删除账号，返回是否存在该账号。
func (a *Authenticator) RemoveAccount(userID uint32) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.accounts[userID]; !ok {
		return false
	}
	delete(a.accounts, userID)
	return true
}
