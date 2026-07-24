package internal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type VaultEntry struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type Vault struct {
	Entries   map[string]VaultEntry
	Path      string
	MasterKey string
}

func NewVault() *Vault {
	home, _ := os.UserHomeDir()
	vaultPath := filepath.Join(home, ".config", "nexus", "vault.enc")

	v := &Vault{
		Entries: make(map[string]VaultEntry),
		Path:    vaultPath,
	}

	if key := os.Getenv("NEXUS_VAULT_KEY"); key != "" {
		v.MasterKey = key
	}

	return v
}

func (v *Vault) SetMasterKey(key string) {
	v.MasterKey = key
}

func (v *Vault) IsLocked() bool {
	return v.MasterKey == ""
}

func (v *Vault) Load() error {
	if v.MasterKey == "" {
		return fmt.Errorf("vault is locked - set NEXUS_VAULT_KEY or use /vault unlock")
	}

	if _, err := os.Stat(v.Path); os.IsNotExist(err) {
		return nil
	}

	encrypted, err := os.ReadFile(v.Path)
	if err != nil {
		return err
	}

	decrypted, err := v.decrypt(string(encrypted))
	if err != nil {
		return fmt.Errorf("decryption failed - wrong key?")
	}

	return json.Unmarshal([]byte(decrypted), &v.Entries)
}

func (v *Vault) Save() error {
	if v.MasterKey == "" {
		return fmt.Errorf("vault is locked")
	}

	data, err := json.MarshalIndent(v.Entries, "", "  ")
	if err != nil {
		return err
	}

	encrypted, err := v.encrypt(string(data))
	if err != nil {
		return err
	}

	dir := filepath.Dir(v.Path)
	os.MkdirAll(dir, 0755)

	return os.WriteFile(v.Path, []byte(encrypted), 0600)
}

func (v *Vault) Get(name string) (string, error) {
	if v.IsLocked() {
		return "", fmt.Errorf("vault is locked")
	}

	entry, ok := v.Entries[name]
	if !ok {
		return "", fmt.Errorf("entry not found: %s", name)
	}

	return entry.Value, nil
}

func (v *Vault) Set(name, value string) error {
	if v.IsLocked() {
		return fmt.Errorf("vault is locked")
	}

	v.Entries[name] = VaultEntry{
		Name:      name,
		Value:     value,
		UpdatedAt: 1000000000,
	}

	return v.Save()
}

func (v *Vault) Delete(name string) error {
	if v.IsLocked() {
		return fmt.Errorf("vault is locked")
	}

	delete(v.Entries, name)
	return v.Save()
}

func (v *Vault) List() []string {
	var names []string
	for name := range v.Entries {
		names = append(names, name)
	}
	return names
}

func (v *Vault) encrypt(text string) (string, error) {
	key := sha256.Sum256([]byte(v.MasterKey))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (v *Vault) decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	key := sha256.Sum256([]byte(v.MasterKey))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (v *Vault) RenderStatus() string {
	if v.IsLocked() {
		return "\033[1;31m🔒 Vault locked\033[0m"
	}

	count := len(v.Entries)
	return fmt.Sprintf("\033[1;32m🔓 Vault unlocked\033[0m (%d entries)", count)
}

func (v *Vault) RenderList() string {
	if v.IsLocked() {
		return "Vault is locked. Use /vault unlock <key>"
	}

	entries := v.List()
	if len(entries) == 0 {
		return "Vault is empty."
	}

	var out strings.Builder
	out.WriteString("\033[1;35mVault entries:\033[0m\n\n")

	for _, name := range entries {
		entry := v.Entries[name]
		masked := v.maskValue(entry.Value)
		out.WriteString(fmt.Sprintf("  • %s: %s\n", name, masked))
	}

	return out.String()
}

func (v *Vault) maskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}
