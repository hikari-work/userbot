package bot

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/utils"
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
	parsedText, entities := utils.ParseHTML(text)

	req := &tg.MessagesSendMessageRequest{
		Peer:        peer,
		Message:     parsedText,
		ReplyMarkup: markup,
		RandomID:    rand.Int63(),
	}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}

	_, err := b.api.MessagesSendMessage(context.Background(), req)
	return err
}

// EditBotMessage mengedit pesan normal yang sudah dikirim bot.
func EditBotMessage(peer tg.InputPeerClass, msgID int, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	parsedText, entities := utils.ParseHTML(text)

	req := &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      msgID,
		Message: parsedText,
	}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}

	if rows != nil {
		req.SetReplyMarkup(buildInlineKeyboard(rows))
	} else {
		req.SetReplyMarkup(&tg.ReplyKeyboardHide{})
	}

	_, err := b.api.MessagesEditMessage(context.Background(), req)
	return err
}

// DeleteBotMessage menghapus pesan normal (bukan inline).
func DeleteBotMessage(peer tg.InputPeerClass, msgID int) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err := b.api.ChannelsDeleteMessages(context.Background(), &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
			ID:      []int{msgID},
		})
		return err
	default:
		_, err := b.api.MessagesDeleteMessages(context.Background(), &tg.MessagesDeleteMessagesRequest{
			ID:     []int{msgID},
			Revoke: true,
		})
		return err
	}
}

// DeleteMessageWithUserbot menghapus pesan menggunakan akun userbot (bisa untuk inline message yang dikirim userbot)
func DeleteMessageWithUserbot(chatID int64, msgID int) error {
	if UserbotClient == nil {
		return fmt.Errorf("userbot client tidak terdaftar")
	}

	ctx := UserbotClient.CreateContext()
	peer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		return err
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = ctx.Raw.ChannelsDeleteMessages(context.Background(), &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
			ID:      []int{msgID},
		})
	default:
		_, err = ctx.Raw.MessagesDeleteMessages(context.Background(), &tg.MessagesDeleteMessagesRequest{
			ID:     []int{msgID},
			Revoke: true,
		})
	}
	return err
}

// EditInlineBotMessage mengedit pesan inline yang dikirim via inline query.
func EditInlineBotMessage(inlineMessageID tg.InputBotInlineMessageIDClass, text string, rows [][]Button) error {
	b := getInstance()
	if b == nil || b.api == nil {
		return fmt.Errorf("bot companion tidak aktif")
	}

	parsedText, entities := utils.ParseHTML(text)

	req := &tg.MessagesEditInlineBotMessageRequest{
		ID:      inlineMessageID,
		Message: parsedText,
	}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}

	if rows != nil {
		req.SetReplyMarkup(buildInlineKeyboard(rows))
	} else {
		req.SetReplyMarkup(&tg.ReplyKeyboardHide{})
	}

	_, err := b.api.MessagesEditInlineBotMessage(context.Background(), req)
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

// BuildInlineKeyboard mengkonversi [][]Button ke tg.ReplyInlineMarkup
func BuildInlineKeyboard(rows [][]Button) *tg.ReplyInlineMarkup {
	return buildInlineKeyboard(rows)
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
