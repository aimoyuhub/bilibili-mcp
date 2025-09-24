package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
)

// Account B站账号信息
type Account struct {
	Name      string    `json:"name"`       // 账号标识名
	Username  string    `json:"username"`   // B站用户名
	Nickname  string    `json:"nickname"`   // 昵称
	UID       string    `json:"uid"`        // B站UID
	Avatar    string    `json:"avatar"`     // 头像URL
	IsDefault bool      `json:"is_default"` // 是否为默认账号
	LoginTime time.Time `json:"login_time"` // 登录时间
	LastUsed  time.Time `json:"last_used"`  // 最后使用时间
	IsActive  bool      `json:"is_active"`  // 是否激活状态
}

// AccountManager 账号管理器
type AccountManager struct {
	configFile string
	cookieDir  string
}

// NewAccountManager 创建账号管理器
func NewAccountManager() *AccountManager {
	cfg := config.Get()
	return &AccountManager{
		configFile: filepath.Join(cfg.Accounts.CookieDir, "accounts.json"),
		cookieDir:  cfg.Accounts.CookieDir,
	}
}

// SaveAccount 保存账号信息
func (am *AccountManager) SaveAccount(account *Account) error {
	// 确保目录存在
	if err := os.MkdirAll(am.cookieDir, 0755); err != nil {
		return errors.Wrap(err, "创建cookies目录失败")
	}

	// 读取现有账号列表
	accounts, _ := am.LoadAccounts()

	// 更新或添加账号
	found := false
	for i, acc := range accounts {
		if acc.Name == account.Name {
			// 保持一些原有信息
			account.LoginTime = acc.LoginTime
			accounts[i] = *account
			found = true
			break
		}
	}

	if !found {
		account.LoginTime = time.Now()
		accounts = append(accounts, *account)
	}

	account.LastUsed = time.Now()

	// 如果这是第一个账号，设为默认
	if len(accounts) == 1 {
		accounts[0].IsDefault = true
	}

	// 保存到文件
	return am.saveAccountsToFile(accounts)
}

// LoadAccounts 加载所有账号
func (am *AccountManager) LoadAccounts() ([]Account, error) {
	if _, err := os.Stat(am.configFile); os.IsNotExist(err) {
		return []Account{}, nil
	}

	data, err := os.ReadFile(am.configFile)
	if err != nil {
		return nil, errors.Wrap(err, "读取账号配置文件失败")
	}

	var accounts []Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, errors.Wrap(err, "解析账号配置文件失败")
	}

	return accounts, nil
}

// GetAccount 获取指定账号
func (am *AccountManager) GetAccount(name string) (*Account, error) {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.Name == name {
			return &acc, nil
		}
	}

	return nil, fmt.Errorf("账号 '%s' 不存在", name)
}

// GetDefaultAccount 获取默认账号
func (am *AccountManager) GetDefaultAccount() (*Account, error) {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.IsDefault && acc.IsActive {
			return &acc, nil
		}
	}

	// 如果没有默认账号，返回第一个激活的账号
	for _, acc := range accounts {
		if acc.IsActive {
			return &acc, nil
		}
	}

	return nil, fmt.Errorf("没有可用的账号，请先登录")
}

// SetDefaultAccount 设置默认账号
func (am *AccountManager) SetDefaultAccount(name string) error {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return err
	}

	found := false
	for i := range accounts {
		if accounts[i].Name == name {
			accounts[i].IsDefault = true
			found = true
		} else {
			accounts[i].IsDefault = false
		}
	}

	if !found {
		return fmt.Errorf("账号 '%s' 不存在", name)
	}

	return am.saveAccountsToFile(accounts)
}

// ActivateAccount 激活账号
func (am *AccountManager) ActivateAccount(name string) error {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return err
	}

	found := false
	for i := range accounts {
		if accounts[i].Name == name {
			accounts[i].IsActive = true
			accounts[i].LastUsed = time.Now()
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("账号 '%s' 不存在", name)
	}

	return am.saveAccountsToFile(accounts)
}

// DeactivateAccount 停用账号
func (am *AccountManager) DeactivateAccount(name string) error {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return err
	}

	found := false
	for i := range accounts {
		if accounts[i].Name == name {
			accounts[i].IsActive = false
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("账号 '%s' 不存在", name)
	}

	return am.saveAccountsToFile(accounts)
}

// GetCookieFile 获取账号的Cookie文件路径
func (am *AccountManager) GetCookieFile(accountName string) string {
	return filepath.Join(am.cookieDir, fmt.Sprintf("%s_bilibili_cookies.json", accountName))
}

// DeleteAccount 删除账号
func (am *AccountManager) DeleteAccount(name string) error {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return err
	}

	// 找到并删除账号
	newAccounts := make([]Account, 0, len(accounts))
	found := false
	for _, acc := range accounts {
		if acc.Name != name {
			newAccounts = append(newAccounts, acc)
		} else {
			found = true
			// 删除对应的cookie文件
			cookieFile := am.GetCookieFile(name)
			os.Remove(cookieFile)
		}
	}

	if !found {
		return fmt.Errorf("账号 '%s' 不存在", name)
	}

	// 如果删除的是默认账号，重新设置默认账号
	if len(newAccounts) > 0 {
		hasDefault := false
		for _, acc := range newAccounts {
			if acc.IsDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			newAccounts[0].IsDefault = true
		}
	}

	return am.saveAccountsToFile(newAccounts)
}

// saveAccountsToFile 保存账号列表到文件
func (am *AccountManager) saveAccountsToFile(accounts []Account) error {
	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return errors.Wrap(err, "序列化账号信息失败")
	}

	return os.WriteFile(am.configFile, data, 0644)
}

// UpdateLastUsed 更新账号最后使用时间
func (am *AccountManager) UpdateLastUsed(name string) error {
	accounts, err := am.LoadAccounts()
	if err != nil {
		return err
	}

	for i := range accounts {
		if accounts[i].Name == name {
			accounts[i].LastUsed = time.Now()
			return am.saveAccountsToFile(accounts)
		}
	}

	return fmt.Errorf("账号 '%s' 不存在", name)
}
