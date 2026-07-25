package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nexus-cli/nexus/config"
)

const (
	UpdateRepo      = "MrModelOS/Nexus"
	UpdateCheckURL  = "https://api.github.com/repos/" + UpdateRepo + "/releases/latest"
	UpdateBinaryURL = "https://github.com/" + UpdateRepo + "/releases/download/%s/nex-%s-%s"
)

var currentVersion = "1.0.0"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type UpdateState struct {
	CurrentVersion string
	LatestVersion  string
	Available      bool
	Downloading    bool
	Progress       int
	Error          error
}

func NewUpdateState() *UpdateState {
	return &UpdateState{
		CurrentVersion: currentVersion,
	}
}

func (us *UpdateState) CheckForUpdate() error {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(UpdateCheckURL)
	if err != nil {
		return fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	us.LatestVersion = strings.TrimPrefix(release.TagName, "v")
	us.Available = us.LatestVersion != "" && us.LatestVersion != us.CurrentVersion

	return nil
}

func (us *UpdateState) DownloadAndInstall() error {
	us.Downloading = true
	us.Error = nil

	platform := runtime.GOOS
	arch := runtime.GOARCH

	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	}

	binaryURL := fmt.Sprintf(UpdateBinaryURL, "v"+us.LatestVersion, platform, arch)

	if platform == "windows" {
		binaryURL += ".exe"
	}

	client := &http.Client{Timeout: 120 * time.Second}

	resp, err := client.Get(binaryURL)
	if err != nil {
		us.Error = fmt.Errorf("download: %w", err)
		us.Downloading = false
		return us.Error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		us.Error = fmt.Errorf("download: status %d", resp.StatusCode)
		us.Downloading = false
		return us.Error
	}

	exePath, err := os.Executable()
	if err != nil {
		us.Error = fmt.Errorf("get executable: %w", err)
		us.Downloading = false
		return us.Error
	}

	exeDir := filepath.Dir(exePath)
	tmpPath := filepath.Join(exeDir, ".nex-update-temp")

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		us.Error = fmt.Errorf("create temp: %w", err)
		us.Downloading = false
		return us.Error
	}

	totalSize := resp.ContentLength
	downloaded := 0
	buf := make([]byte, 32*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			written, writeErr := tmpFile.Write(buf[:n])
			if writeErr != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				us.Error = fmt.Errorf("write: %w", writeErr)
				us.Downloading = false
				return us.Error
			}
			downloaded += written

			if totalSize > 0 {
				us.Progress = downloaded * 100 / int(totalSize)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			us.Error = fmt.Errorf("read: %w", readErr)
			us.Downloading = false
			return us.Error
		}
	}

	tmpFile.Close()

	if platform != "windows" {
		os.Chmod(tmpPath, 0755)
	}

	backupPath := exePath + ".bak"
	os.Remove(backupPath)

	if err := os.Rename(exePath, backupPath); err != nil {
		os.Remove(tmpPath)
		us.Error = fmt.Errorf("backup: %w", err)
		us.Downloading = false
		return us.Error
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		os.Rename(backupPath, exePath)
		us.Error = fmt.Errorf("install: %w", err)
		us.Downloading = false
		return us.Error
	}

	currentVersion = us.LatestVersion
	us.CurrentVersion = us.LatestVersion
	us.Available = false
	us.Downloading = false

	return nil
}

func (us *UpdateState) Render() string {
	var out strings.Builder

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	labelS := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valueS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	okS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	warnS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	errS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

	out.WriteString(titleS.Render("📦 Nexus Update") + "\n\n")
	out.WriteString(labelS.Render("Current version: ") + valueS.Render(us.CurrentVersion) + "\n")

	if us.Error != nil {
		out.WriteString(errS.Render("Error: "+us.Error.Error()) + "\n\n")
		out.WriteString(labelS.Render("Try downloading manually from:") + "\n")
		out.WriteString("  https://github.com/" + UpdateRepo + "/releases\n")
	} else if us.Downloading {
		out.WriteString(labelS.Render("Downloading update...") + "\n")
		bar := strings.Repeat("█", us.Progress/2) + strings.Repeat("░", 50-us.Progress/2)
		out.WriteString("  " + valueS.Render(bar) + " " + valueS.Render(fmt.Sprintf("%d%%", us.Progress)) + "\n")
	} else if us.Available {
		out.WriteString(labelS.Render("Latest version: ") + okS.Render(us.LatestVersion) + "\n\n")
		out.WriteString(warnS.Render("⚡ Update available!") + "\n\n")
		out.WriteString(labelS.Render("Run ") + valueS.Render("/update") + labelS.Render(" to install") + "\n")
		out.WriteString(labelS.Render("Your config and data are safe in ~/.config/nexus/") + "\n")
	} else {
		out.WriteString(labelS.Render("Latest version: ") + okS.Render(us.LatestVersion) + "\n\n")
		out.WriteString(okS.Render("✓ You're up to date!") + "\n")
	}

	return out.String()
}

func GetUpdateHint() string {
	us := NewUpdateState()
	if err := us.CheckForUpdate(); err != nil {
		return ""
	}
	if us.Available {
		return fmt.Sprintf("\033[1;33m📦 Update available: %s → %s (run /update)\033[0m", us.CurrentVersion, us.LatestVersion)
	}
	return ""
}

func GetVersion() string {
	return currentVersion
}

func CheckAndUpdateConfig(cfg *config.Config) *config.Config {
	if cfg.Pool == nil {
		cfg.Pool = &config.ModelPool{
			Enabled:    false,
			MaxRetries: 3,
			RetryDelay: 1000,
		}
	}
	if cfg.Context == nil {
		cfg.Context = &config.ContextConfig{
			MaxTokens:      100000,
			AutoCompact:    true,
			CompactPercent: 80,
		}
	}
	return cfg
}

func RunUpdateCheck() *UpdateState {
	us := NewUpdateState()
	us.CheckForUpdate()
	return us
}

func SelfUpdate() error {
	us := NewUpdateState()
	if err := us.CheckForUpdate(); err != nil {
		return err
	}
	if !us.Available {
		return fmt.Errorf("already up to date (v%s)", us.CurrentVersion)
	}
	return us.DownloadAndInstall()
}

func RestoreBackup() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	backupPath := exePath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	return os.Rename(backupPath, exePath)
}

func CleanupBackup() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	os.Remove(exePath + ".bak")
}

func init() {
	go func() {
		time.Sleep(5 * time.Second)
		us := RunUpdateCheck()
		if us.Available {
			_ = exec.Command("notify-send", "Nexus Update", fmt.Sprintf("v%s available → v%s", us.CurrentVersion, us.LatestVersion)).Run()
		}
	}()
}
