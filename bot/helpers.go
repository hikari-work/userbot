package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Button merepresentasikan satu tombol pada inline keyboard
type Button struct {
	Text         string // teks yang tampil di tombol
	CallbackData string // data yang dikirim saat tombol ditekan (max 64 byte)
	URL          string // URL yang dibuka saat tombol ditekan
	SwitchInline string // beralih ke inline mode dengan query ini
}

// Structs untuk request Bot API HTTP
type botAPIInlineKeyboardButton struct {
	Text         string  `json:"text"`
	CallbackData *string `json:"callback_data,omitempty"`
	URL          *string `json:"url,omitempty"`
	SwitchInline *string `json:"switch_inline_query,omitempty"`
}

type botAPIInlineKeyboardMarkup struct {
	InlineKeyboard [][]botAPIInlineKeyboardButton `json:"inline_keyboard"`
}

type sendMessageRequest struct {
	ChatID      int64                       `json:"chat_id"`
	Text        string                      `json:"text"`
	ParseMode   string                      `json:"parse_mode"`
	ReplyMarkup *botAPIInlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type editMessageTextRequest struct {
	ChatID      int64                       `json:"chat_id"`
	MessageID   int                         `json:"message_id"`
	Text        string                      `json:"text"`
	ParseMode   string                      `json:"parse_mode"`
	ReplyMarkup *botAPIInlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type answerCallbackQueryRequest struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}

// Client HTTP dengan timeout
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// SendWithButtons mengirim pesan teks dengan inline keyboard via bot.
// chatID: ID chat tujuan (e.g. -1001234567890).
// rows: baris tombol, tiap row berisi beberapa Button.
func SendWithButtons(chatID int64, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	reqBody := sendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}
	if rows != nil {
		reqBody.ReplyMarkup = buildHTTPInlineKeyboard(rows)
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.cfg.BotToken)
	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errMsg struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errMsg)
		return fmt.Errorf("telegram bot api error: %s (code %d)", errMsg.Description, resp.StatusCode)
	}
	return nil
}

// EditBotMessage mengedit pesan yang sudah dikirim bot.
// Jika rows nil, keyboard dihapus.
func EditBotMessage(chatID int64, msgID int, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	reqBody := editMessageTextRequest{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
		ParseMode: "HTML",
	}
	if rows != nil {
		reqBody.ReplyMarkup = buildHTTPInlineKeyboard(rows)
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", b.cfg.BotToken)
	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errMsg struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errMsg)
		return fmt.Errorf("telegram bot api error: %s (code %d)", errMsg.Description, resp.StatusCode)
	}
	return nil
}

// AnswerCallbackQuery menjawab callback query dari tombol yang ditekan user.
// Jika showAlert true, teks ditampilkan sebagai popup alert (bukan toast kecil).
func AnswerCallbackQuery(ctx context.Context, queryID int64, text string, showAlert bool) error {
	b := getInstance()
	if b == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	reqBody := answerCallbackQueryRequest{
		CallbackQueryID: fmt.Sprintf("%d", queryID),
		Text:            text,
		ShowAlert:       showAlert,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", b.cfg.BotToken)
	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errMsg struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errMsg)
		return fmt.Errorf("telegram bot api error: %s (code %d)", errMsg.Description, resp.StatusCode)
	}
	return nil
}

// IsActive mengembalikan true jika Bot Companion aktif dan token diset
func IsActive() bool {
	b := getInstance()
	return b != nil && b.cfg.BotToken != ""
}

// ── helpers internal ──────────────────────────────────────────────────────────

// buildHTTPInlineKeyboard mengkonversi [][]Button ke format Bot API
func buildHTTPInlineKeyboard(rows [][]Button) *botAPIInlineKeyboardMarkup {
	var httpRows [][]botAPIInlineKeyboardButton
	for _, row := range rows {
		var httpRow []botAPIInlineKeyboardButton
		for _, btn := range row {
			var httpBtn botAPIInlineKeyboardButton
			httpBtn.Text = btn.Text
			if btn.CallbackData != "" {
				val := btn.CallbackData
				httpBtn.CallbackData = &val
			} else if btn.URL != "" {
				val := btn.URL
				httpBtn.URL = &val
			} else if btn.SwitchInline != "" {
				val := btn.SwitchInline
				httpBtn.SwitchInline = &val
			} else {
				val := btn.Text
				httpBtn.CallbackData = &val
			}
			httpRow = append(httpRow, httpBtn)
		}
		httpRows = append(httpRows, httpRow)
	}
	return &botAPIInlineKeyboardMarkup{InlineKeyboard: httpRows}
}
