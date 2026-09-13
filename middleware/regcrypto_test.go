package middleware

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// crossRepoSecret 与 crossRepoVector 是与 reg-to 共享的一致性测试向量。
//
// 两边仓库都用同一份密文断言解密结果，任何一侧改动密钥派生、nonce 布局或编码格式
// 都会让这个测试失败，从而在 CI 阶段拦住跨仓库不兼容。
const (
	crossRepoSecret = "cross-repo-test-secret-0123456789"
	crossRepoVector = "v1:xFFq9PMnC5rx6qxWFQmHPuYliwMVkJ90tB1zN1skds3ShfMO4PUOC3FLsvVAxF8"
)

// TestDecryptCrossRepoVector 验证与 reg-to 共享的密文能被正确解开，防止两侧格式漂移。
func TestDecryptCrossRepoVector(t *testing.T) {
	plaintext, err := DecryptPassword(crossRepoSecret, crossRepoVector)
	require.NoError(t, err, "解密共享测试向量失败，说明与 reg-to 的格式已不一致")
	assert.Equal(t, "P@ssw0rd-测试🔐", plaintext)
}

// TestEncryptDecryptRoundTrip 验证加解密往返还原原文，且密文中不出现明文。
func TestEncryptDecryptRoundTrip(t *testing.T) {
	for _, password := range []string{
		"simple123",
		"P@ssw0rd-测试🔐",
		strings.Repeat("x", 512),
	} {
		ciphertext, err := EncryptPassword("test-encryption-secret-0123456789", password)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(ciphertext, encPasswordPrefix))

		plaintext, err := DecryptPassword("test-encryption-secret-0123456789", ciphertext)
		require.NoError(t, err)
		assert.Equal(t, password, plaintext)
	}
}

// TestEncryptUsesFreshNonce 验证相同明文两次加密产生不同密文。
func TestEncryptUsesFreshNonce(t *testing.T) {
	first, err := EncryptPassword("test-encryption-secret-0123456789", "same-password")
	require.NoError(t, err)

	second, err := EncryptPassword("test-encryption-secret-0123456789", "same-password")
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "相同明文两次加密应产生不同密文")
}

// TestDecryptRejectsInvalidInput 验证密钥、格式、编码任一不合法时均拒绝。
func TestDecryptRejectsInvalidInput(t *testing.T) {
	cases := map[string]struct {
		secret  string
		encoded string
	}{
		"密钥未配置":     {"", crossRepoVector},
		"密钥不匹配":     {"another-encryption-secret-012345678", crossRepoVector},
		"缺少版本前缀":    {crossRepoSecret, strings.TrimPrefix(crossRepoVector, "v1:")},
		"版本不受支持":    {crossRepoSecret, "v9:abcdef"},
		"Base64 非法": {crossRepoSecret, "v1:!!!!"},
		"密文过短":      {crossRepoSecret, "v1:AAAA"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecryptPassword(tc.secret, tc.encoded)
			assert.Error(t, err, "非法输入应返回错误")
		})
	}
}

// TestDecryptRejectsTamperedCiphertext 验证被篡改的密文会被 GCM 认证拒绝。
func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	ciphertext, err := EncryptPassword("test-encryption-secret-0123456789", "password123")
	require.NoError(t, err)

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, encPasswordPrefix))
	require.NoError(t, err)

	// 翻转最后一个字节（属于 GCM 认证标签），认证必须失败。
	payload[len(payload)-1] ^= 0xFF

	_, err = DecryptPassword("test-encryption-secret-0123456789", encPasswordPrefix+base64.RawURLEncoding.EncodeToString(payload))
	assert.Error(t, err, "被篡改的密文应认证失败")
}

// TestEncryptRejectsEmptySecret 验证共享密钥未配置时不得进行加解密。
func TestEncryptRejectsEmptySecret(t *testing.T) {
	_, err := EncryptPassword("", "password")
	assert.ErrorIs(t, err, ErrSecretNotConfigured)

	_, err = DecryptPassword("", crossRepoVector)
	assert.ErrorIs(t, err, ErrSecretNotConfigured)
}
