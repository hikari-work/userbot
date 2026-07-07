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
// peer: gunakan ctx.ResolveInputPeerById(chatID) dari handler userbot.
// rows: baris tombol, tiap row berisi beberapa Button.
func SendWithButtons(peer tg.InputPeerClass, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	markup := buildInlineKeyboard(rows)

	_, err := b.api.MessagesSendMessage(context.Background(), &tg.MessagesSendMessageRequest{
		Peer:        peer,
		Message:     text,
		ReplyMarkup: markup,
		RandomID:    rand.Int63(),
	})
	return err
}

// EditBotMessage mengedit pesan yang sudah dikirim bot.
// Jika rows nil, keyboard dihapus (pesan menjadi plain text).
func EditBotMessage(peer tg.InputPeerClass, msgID int, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	req := &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      msgID,
		Message: text,
	}
	if rows != nil {
		req.SetReplyMarkup(buildInlineKeyboard(rows))
	} else {
		// Hapus keyboard
		req.SetReplyMarkup(&tg.ReplyKeyboardHide{})
	}

	_, err := b.api.MessagesEditMessage(context.Background(), req)
	return err
}

// AnswerCallbackQuery menjawab callback query dari tombol yang ditekan user.
// Jika showAlert true, teks ditampilkan sebagai popup alert (bukan toast kecil).
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

// PeerFromCallbackQuery mengkonversi PeerClass dari callback query ke InputPeerClass
// menggunakan entity store yang terisi dari update sebelumnya.
func PeerFromCallbackQuery(peer tg.PeerClass) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerUser:
		hash := entityStore.resolveUserHash(p.UserID)
		return &tg.InputPeerUser{UserID: p.UserID, AccessHash: hash}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		hash := entityStore.resolveChannelHash(p.ChannelID)
		return &tg.InputPeerChannel{ChannelID: p.ChannelID, AccessHash: hash}
	}
	return &tg.InputPeerEmpty{}
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
