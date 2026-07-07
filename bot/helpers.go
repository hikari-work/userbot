package bot

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gotd/td/tg"
)

// Button merepresentasikan satu tombol pada inline keyboard
type Button struct {
	Text         string // teks yang tampil di tombol
	CallbackData string // data yang dikirim saat tombol ditekan (max 64 byte)
	URL          string // URL yang dibuka saat tombol ditekan
	SwitchInline string // beralih ke inline mode dengan query ini
}

// SendWithButtons mengirim pesan teks dengan inline keyboard via bot.
// rows adalah baris tombol, setiap row berisi beberapa Button.
// text mendukung HTML entity (bold, italic, code, dll.) via Telegram MessageEntity.
func SendWithButtons(chatID int64, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	markup := buildInlineKeyboard(rows)

	_, err := b.api.MessagesSendMessage(context.Background(), &tg.MessagesSendMessageRequest{
		Peer:        inputPeerFromID(chatID),
		Message:     text,
		ReplyMarkup: markup,
		RandomID:    rand.Int63(),
	})
	return err
}

// EditBotMessage mengedit pesan yang sudah dikirim bot, dengan inline keyboard baru.
func EditBotMessage(chatID int64, msgID int, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	req := &tg.MessagesEditMessageRequest{
		Peer:    inputPeerFromID(chatID),
		ID:      msgID,
		Message: text,
	}
	if rows != nil {
		req.SetReplyMarkup(buildInlineKeyboard(rows))
	}

	_, err := b.api.MessagesEditMessage(context.Background(), req)
	return err
}

// AnswerCallbackQuery menjawab callback query dari tombol yang ditekan user.
// Jika showAlert true, teks akan ditampilkan sebagai popup alert bukan toast.
func AnswerCallbackQuery(ctx context.Context, queryID int64, text string, showAlert bool) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	req := &tg.MessagesSetBotCallbackAnswerRequest{
		QueryID: queryID,
	}
	if text != "" {
		req.SetMessage(text)
		req.SetAlert(showAlert)
	}

	_, err := b.api.MessagesSetBotCallbackAnswer(ctx, req)
	return err
}

// AnswerInlineQuery menjawab inline query dengan daftar hasil.
func AnswerInlineQuery(ctx context.Context, queryID int64, results []tg.InputBotInlineResultClass) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	_, err := b.api.MessagesSetInlineBotResults(ctx, &tg.MessagesSetInlineBotResultsRequest{
		QueryID:    queryID,
		Results:    results,
		CacheTime:  300,
		NextOffset: "",
	})
	return err
}

// IsActive mengembalikan true jika Bot Companion aktif dan sudah terautentikasi
func IsActive() bool {
	b := getInstance()
	return b != nil && b.api != nil
}

// ── helpers internal ──────────────────────────────────────────────────────────

// buildInlineKeyboard mengkonversi [][]Button ke tg.ReplyInlineMarkup
func buildInlineKeyboard(rows [][]Button) *tg.ReplyInlineMarkup {
	var tgRows []tg.KeyboardButtonRow
	for _, row := range rows {
		var tgBtns []tg.KeyboardButtonClass
		for _, btn := range row {
			switch {
			case btn.CallbackData != "":
				tgBtns = append(tgBtns, &tg.KeyboardButtonCallback{
					Text: btn.Text,
					Data: []byte(btn.CallbackData),
				})
			case btn.URL != "":
				tgBtns = append(tgBtns, &tg.KeyboardButtonURL{
					Text: btn.Text,
					URL:  btn.URL,
				})
			case btn.SwitchInline != "":
				tgBtns = append(tgBtns, &tg.KeyboardButtonSwitchInline{
					Text:  btn.Text,
					Query: btn.SwitchInline,
				})
			default:
				tgBtns = append(tgBtns, &tg.KeyboardButtonCallback{
					Text: btn.Text,
					Data: []byte(btn.Text),
				})
			}
		}
		tgRows = append(tgRows, tg.KeyboardButtonRow{Buttons: tgBtns})
	}
	return &tg.ReplyInlineMarkup{Rows: tgRows}
}

// inputPeerFromID membuat InputPeer dari chatID numerik Telegram.
// Mendukung: user (> 0), basic group (< 0, kecil), supergroup/channel (< -1_000_000_000_000).
func inputPeerFromID(chatID int64) tg.InputPeerClass {
	if chatID > 0 {
		return &tg.InputPeerUser{UserID: chatID}
	}
	if chatID < -1_000_000_000_000 {
		channelID := -(chatID + 1_000_000_000_000)
		return &tg.InputPeerChannel{ChannelID: channelID}
	}
	return &tg.InputPeerChat{ChatID: -chatID}
}
