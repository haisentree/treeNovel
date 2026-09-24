// Package admin 提供后台会话认证：内存 session + cookie，
// 密码取环境变量 ADMIN_PASSWORD（默认 admin），重启后需重新登录。
package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	cookieName = "tn_admin"
	sessionTTL = 24 * time.Hour
)

var (
	mu       sync.Mutex
	password = "admin"
	sessions = map[string]time.Time{} // token -> 过期时间
)

// SetPassword 设置后台登录密码，空串保持默认。
func SetPassword(p string) {
	if p != "" {
		password = p
	}
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "" // crypto/rand 失败极罕见，返回空串使本次登录失败
	}
	return hex.EncodeToString(b)
}

func valid(token string) bool {
	mu.Lock()
	defer mu.Unlock()
	exp, ok := sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(sessions, token)
		return false
	}
	return true
}

// Login 校验密码，成功则种下会话 cookie。
func Login(c *gin.Context, pass string) bool {
	if pass == "" || pass != password {
		return false
	}
	token := newToken()
	if token == "" {
		return false
	}
	mu.Lock()
	sessions[token] = time.Now().Add(sessionTTL)
	mu.Unlock()
	c.SetCookie(cookieName, token, int(sessionTTL.Seconds()), "/", "", false, true)
	return true
}

// Logout 销毁会话。
func Logout(c *gin.Context) {
	if token, err := c.Cookie(cookieName); err == nil {
		mu.Lock()
		delete(sessions, token)
		mu.Unlock()
	}
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
}

// Require 会话校验中间件，未登录重定向到登录页。
func Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := c.Cookie(cookieName); err == nil && valid(token) {
			c.Next()
			return
		}
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
	}
}
