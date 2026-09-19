package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"AstraScheduleServerGo/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRegSecret = "reg-token-test-secret-0123456789ab"

// setRegSecret 在测试期间替换共享密钥，结束后还原。
func setRegSecret(t *testing.T, secret string) {
	t.Helper()

	previous := model.Configs.Internal.Secret
	model.Configs.Internal.Secret = secret
	t.Cleanup(func() { model.Configs.Internal.Secret = previous })
}

// signRegToken 用给定 claims 签发一个注册令牌。
func signRegToken(t *testing.T, secret string, claims *RegClaims) string {
	t.Helper()

	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Minute))
	}
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(time.Now())
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// callRegisterTenant 通过中间件访问一个只回显口令的端点。
func callRegisterTenant(token string) *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/probe", RegTokenAuth(), func(c *gin.Context) {
		password, ok := GetRegPassword(c)
		claims := GetRegClaims(c)
		subdomain := ""
		if claims != nil {
			subdomain = claims.Subdomain
		}
		c.JSON(http.StatusOK, gin.H{"password": password, "resolved": ok, "subdomain": subdomain})
	})

	request := httptest.NewRequest(http.MethodPost, "/probe", nil)
	if token != "" {
		request.Header.Set("X-Reg-Token", token)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestRegTokenAuthResolvesEncryptedPassword 验证密文令牌被解出口令并经 Context 传给 handler。
func TestRegTokenAuthResolvesEncryptedPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setRegSecret(t, testRegSecret)

	encrypted, err := EncryptPassword(testRegSecret, "password123")
	require.NoError(t, err)

	token := signRegToken(t, testRegSecret, &RegClaims{
		Subdomain:   "nj39",
		Username:    "admin",
		EncPassword: encrypted,
	})

	recorder := callRegisterTenant(token)
	require.Equal(t, http.StatusOK, recorder.Code)

	payload := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "password123", payload["password"])
	assert.Equal(t, true, payload["resolved"])
	assert.Equal(t, "nj39", payload["subdomain"])
}

// 旧版注册站会把明文口令写进 password 字段，升级窗口期内仍需可用。
// TestRegTokenAuthAcceptsLegacyPlaintextPassword 验证旧版明文令牌在升级窗口期内仍然可用。
func TestRegTokenAuthAcceptsLegacyPlaintextPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setRegSecret(t, testRegSecret)

	token := signRegToken(t, testRegSecret, &RegClaims{
		Subdomain: "nj39",
		Username:  "admin",
		Password:  "legacy-password",
	})

	recorder := callRegisterTenant(token)
	require.Equal(t, http.StatusOK, recorder.Code)

	payload := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "legacy-password", payload["password"])
}

// TestRegTokenAuthRejectsBadTokens 验证各类非法令牌一律返回 401。
func TestRegTokenAuthRejectsBadTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setRegSecret(t, testRegSecret)

	encrypted, err := EncryptPassword(testRegSecret, "password123")
	require.NoError(t, err)

	otherEncrypted, err := EncryptPassword("some-other-secret-0123456789abcdef", "password123")
	require.NoError(t, err)

	cases := map[string]string{
		"缺少令牌":     "",
		"结构非法":     "aaa.bbb.ccc",
		"签名不匹配":    signRegToken(t, "wrong-signing-secret-0123456789abc", &RegClaims{Subdomain: "nj39", EncPassword: encrypted}),
		"既无密文也无明文": signRegToken(t, testRegSecret, &RegClaims{Subdomain: "nj39"}),
		"密文无法解密":   signRegToken(t, testRegSecret, &RegClaims{Subdomain: "nj39", EncPassword: otherEncrypted}),
		"密文格式不受支持": signRegToken(t, testRegSecret, &RegClaims{Subdomain: "nj39", EncPassword: "v9:abcdef"}),
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := callRegisterTenant(token)
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

// TestResolvePasswordPrefersEncryptedField 验证密文与明文同时存在时优先使用密文。
// TestRegTokenAuthRejectsTokenWithoutExpiration 验证缺少 exp 的令牌被拒绝。
//
// jwt/v5 默认不要求 exp：签名合法但没有过期时间的令牌会被永久接受，
// 并反复进入建租户流程。
func TestRegTokenAuthRejectsTokenWithoutExpiration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setRegSecret(t, testRegSecret)

	encrypted, err := EncryptPassword(testRegSecret, "password123")
	require.NoError(t, err)

	claims := &RegClaims{Subdomain: "nj39", EncPassword: encrypted}
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	// 故意不设置 ExpiresAt。

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testRegSecret))
	require.NoError(t, err)

	recorder := callRegisterTenant(signed)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

// TestRegTokenAuthRejectsShortSecret 验证密钥缺失或过短时一律拒绝。
//
// 空密钥下的 HMAC 任何人都能伪造，绝不能拿去验签。
func TestRegTokenAuthRejectsShortSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for name, secret := range map[string]string{"空密钥": "", "过短密钥": "short"} {
		t.Run(name, func(t *testing.T) {
			encrypted, err := EncryptPassword(testRegSecret, "password123")
			require.NoError(t, err)

			claims := &RegClaims{Subdomain: "nj39", EncPassword: encrypted}
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Minute))
			claims.IssuedAt = jwt.NewNumericDate(time.Now())

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signed, err := token.SignedString([]byte(secret))
			require.NoError(t, err)

			setRegSecret(t, secret)
			recorder := callRegisterTenant(signed)
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}
func TestResolvePasswordPrefersEncryptedField(t *testing.T) {
	encrypted, err := EncryptPassword(testRegSecret, "from-ciphertext")
	require.NoError(t, err)

	claims := &RegClaims{EncPassword: encrypted, Password: "from-plaintext"}

	password, err := claims.ResolvePassword(testRegSecret)
	require.NoError(t, err)
	assert.Equal(t, "from-ciphertext", password, "应优先使用密文字段")
}

// TestResolvePasswordRejectsEmptyClaims 验证两种口令字段都缺失时返回错误。
// TestResolvePasswordRejectsEmptyDecryptedPassword 验证解密出空口令时返回错误。
//
// bcrypt 接受空密码，若放行会静默创建出空口令管理员，而不是让注册失败。
func TestResolvePasswordRejectsEmptyDecryptedPassword(t *testing.T) {
	encrypted, err := EncryptPassword(testRegSecret, "")
	require.NoError(t, err)

	claims := &RegClaims{EncPassword: encrypted}

	_, err = claims.ResolvePassword(testRegSecret)
	assert.Error(t, err, "空口令必须被拒绝")
}

// TestRegTokenAuthRejectsEmptyPasswordToken 验证空口令令牌过不了中间件。
func TestRegTokenAuthRejectsEmptyPasswordToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setRegSecret(t, testRegSecret)

	encrypted, err := EncryptPassword(testRegSecret, "")
	require.NoError(t, err)

	token := signRegToken(t, testRegSecret, &RegClaims{
		Subdomain:   "nj39",
		EncPassword: encrypted,
	})

	recorder := callRegisterTenant(token)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
func TestResolvePasswordRejectsEmptyClaims(t *testing.T) {
	_, err := (&RegClaims{}).ResolvePassword(testRegSecret)
	assert.Error(t, err)
}
