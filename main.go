package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// --- Configuration Structures ---

type Config struct {
	Telegram TelegramConfig `toml:"telegram"`
	Profiles []Profile      `toml:"profiles"`
}

type TelegramConfig struct {
	Enabled  bool   `toml:"enabled"`
	BotToken string `toml:"bot_token"`
	ChatID   string `toml:"chat_id"`
}

type Profile struct {
	AccountName string `toml:"account_name"`
	Cred        string `toml:"cred"`
	SkGameRole  string `toml:"sk_game_role"`
	Platform    string `toml:"platform"`
	VName       string `toml:"v_name"`
}

// --- API Response Structures (Same as before) ---

type ApiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type AwardData struct {
	AwardIds        []AwardItem             `json:"awardIds"`
	ResourceInfoMap map[string]ResourceInfo `json:"resourceInfoMap"`
}

type AwardItem struct {
	ID string `json:"id"`
}

type ResourceInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type RefreshData struct {
	Token string `json:"token"`
}

type Result struct {
	Name    string
	Success bool
	Status  string
	Rewards string
}

// --- Constants ---

const (
	AttendanceURL = "https://zonai.skport.com/web/v1/game/endfield/attendance"
	RefreshURL    = "https://zonai.skport.com/web/v1/auth/refresh"
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:147.0) Gecko/20100101 Firefox/147.0"
)

func main() {
	// 1. Load Config
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.toml"
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Println("Starting Daily Check-in...")

	// 2. Run Checks in Parallel
	var wg sync.WaitGroup
	results := make([]Result, len(cfg.Profiles))

	for i, p := range cfg.Profiles {
		wg.Add(1)
		go func(idx int, profile Profile) {
			defer wg.Done()
			results[idx] = processProfile(profile)
		}(i, p)
	}

	wg.Wait()

	// 3. Generate Report
	log.Println("\n--- Final Report ---")
	for _, r := range results {
		log.Printf("[%s] %s | Rewards: %s", r.Name, r.Status, r.Rewards)
	}

	// 4. Send Telegram Notification
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		sendTelegram(cfg.Telegram, results)
	}
}

func processProfile(p Profile) Result {
	log.Printf("[%s] Checking credentials...", p.AccountName)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Refresh Token
	token, err := refreshToken(p)
	if err != nil {
		log.Printf("[%s] Token refresh warning: %v", p.AccountName, err)
		token = "" 
	} else {
		log.Printf("[%s] Token refreshed.", p.AccountName)
	}

	// Generate Signature
	sign := generateSign("/web/v1/game/endfield/attendance", "", timestamp, token, p.Platform, p.VName)

	// Prepare Request
	req, _ := http.NewRequest("POST", AttendanceURL, nil)
	setHeaders(req, p, timestamp, sign)

	// Execute Request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	
	res := Result{Name: p.AccountName, Success: false}

	if err != nil {
		res.Status = "💥 Exception"
		res.Rewards = err.Error()
		return res
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var apiResp ApiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		res.Status = "💥 Parse Error"
		res.Rewards = err.Error()
		return res
	}

	log.Printf("[%s] API Code: %d", p.AccountName, apiResp.Code)

	if apiResp.Code == 0 {
		res.Success = true
		res.Status = "✅ Check-in Successful"
		
		var data AwardData
		if err := json.Unmarshal(apiResp.Data, &data); err == nil && len(data.AwardIds) > 0 {
			var rewards []string
			for _, award := range data.AwardIds {
				if info, ok := data.ResourceInfoMap[award.ID]; ok {
					rewards = append(rewards, fmt.Sprintf("%s x%d", info.Name, info.Count))
				} else {
					rewards = append(rewards, award.ID)
				}
			}
			res.Rewards = strings.Join(rewards, ", ")
		} else {
			res.Rewards = "No detailed reward info."
		}

	} else if apiResp.Code == 10001 {
		res.Success = true
		res.Status = "👌 Already Checked In"
		res.Rewards = "Nothing to claim"
	} else {
		res.Status = fmt.Sprintf("❌ Error (%d)", apiResp.Code)
		res.Rewards = apiResp.Message
	}

	return res
}

func refreshToken(p Profile) (string, error) {
	req, _ := http.NewRequest("GET", RefreshURL, nil)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("cred", p.Cred)
	req.Header.Set("platform", p.Platform)
	req.Header.Set("vName", p.VName)
	req.Header.Set("Origin", "https://game.skport.com")
	req.Header.Set("Referer", "https://game.skport.com/")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var apiResp ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", err
	}

	if apiResp.Code == 0 {
		var data RefreshData
		if err := json.Unmarshal(apiResp.Data, &data); err == nil {
			return data.Token, nil
		}
	}
	return "", fmt.Errorf("code %d: %s", apiResp.Code, apiResp.Message)
}

func generateSign(path, body, timestamp, token, platform, vName string) string {
	headerJson := fmt.Sprintf(`{"platform":"%s","timestamp":"%s","dId":"","vName":"%s"}`, platform, timestamp, vName)
	str := path + body + timestamp + headerJson

	h := hmac.New(sha256.New, []byte(token))
	h.Write([]byte(str))
	hmacHex := hex.EncodeToString(h.Sum(nil))

	m := md5.Sum([]byte(hmacHex))
	return hex.EncodeToString(m[:])
}

func setHeaders(req *http.Request, p Profile, timestamp, sign string) {
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://game.skport.com/")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("sk-language", "en")
	req.Header.Set("sk-game-role", p.SkGameRole)
	req.Header.Set("cred", p.Cred)
	req.Header.Set("platform", p.Platform)
	req.Header.Set("vName", p.VName)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", sign)
	req.Header.Set("Origin", "https://game.skport.com")
	req.Header.Set("Connection", "keep-alive")
}

func sendTelegram(cfg TelegramConfig, results []Result) {
	allSuccess := true
	for _, r := range results {
		if !r.Success {
			allSuccess = false
			break
		}
	}

	icon := "🔴"
	if allSuccess {
		icon = "🟢"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s <b>Endfield Daily Check-in Report</b>\n", icon))
	sb.WriteString(fmt.Sprintf("<i>%s (UTC)</i>\n\n", time.Now().UTC().Format("2006-01-02 15:04:05")))

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("👤 <b>%s</b>\n", r.Name))
		sb.WriteString(fmt.Sprintf("<b>Status:</b> %s\n", r.Status))
		sb.WriteString(fmt.Sprintf("<b>Rewards:</b> %s\n", r.Rewards))
		sb.WriteString("-----------------------------\n")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	payload := map[string]interface{}{
		"chat_id":                  cfg.ChatID,
		"text":                     sb.String(),
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}

	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Telegram Error: %v", err)
		return
	}
	defer resp.Body.Close()
}

func loadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	
	if err := toml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
