package json

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/bot"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:          "Json",
		Description:   "Create Json From Message Or Chat",
		Commands:      []string{"json"},
		OnlyOut:       true,
		Handler:       jsonHandler,
		InlineHandler: jsonInlineHandler,
	})
}

func jsonHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	bgCtx := *ctx
	bgCtx.Context = context.Background()

	go func() {
		var data any
		var err error
		hasData := false

		args := update.Args()
		if len(args) > 1 {
			userId := args[1]
			user, err := strconv.Atoi(userId)
			if err != nil {
				if strings.HasPrefix(userId, "@") {
					username, err := bgCtx.ResolveUsername(userId)
					if err != nil {
						_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.GetID(), i18n.Localize("JsonInvalidChatID", nil, nil))
						return
					}
					data = username
				}
			}
			id, err := bgCtx.ResolveInputPeerById(int64(user))
			if err != nil {
				_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.GetID(), i18n.Localize("JsonChatIDNotFound", nil, nil))
				return
			}
			data = id
			hasData = true
		}

		if replyHeader, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader); ok && replyHeader.ReplyToMsgID != 0 {
			var msgs []tg.MessageClass
			msgs, err = bgCtx.GetMessages(uChat.GetID(), []tg.InputMessageClass{&tg.InputMessageID{
				ID: replyHeader.ReplyToMsgID,
			}})
			if err != nil {
				_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.ID, i18n.Localize("JsonErrorGeneral", map[string]any{"Error": html.EscapeString(err.Error())}, nil))
				return
			}
			if len(msgs) == 0 {
				_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.ID, i18n.Localize("JsonMessageNotFound", nil, nil))
				return
			}
			data = msgs[0]
			hasData = true
		}

		if !hasData {
			if uChat.IsAUser() {
				user := update.EffectiveUser()
				if user == nil {
					_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.ID, i18n.Localize("JsonUserNotFound", nil, nil))
					return
				}
				data = user
			} else {
				data = uChat
			}
		}

		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.ID, i18n.Localize("JsonSerializeError", map[string]any{"Error": html.EscapeString(err.Error())}, nil))
			return
		}

		jsonDataStr := string(jsonData)
		runes := []rune(jsonDataStr)

		if len(runes) <= 100 {
			escapedJSON := html.EscapeString(jsonDataStr)
			response := fmt.Sprintf("<pre><code class=\"language-json\">%s</code></pre>", escapedJSON)
			_, err = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.ID, response)
			return
		}

		pasteURL, err := uploadToPasteRS(jsonDataStr)
		if err != nil {
			_, _ = utils.EditMessageHTML(&bgCtx, uChat.GetID(), uMsg.ID, i18n.Localize("JsonUploadPasteError", map[string]any{"Error": html.EscapeString(err.Error())}, nil))
			return
		}

		truncatedJSON := string(runes[:100]) + "\n" + i18n.Localize("JsonTruncated", nil, nil)
		escapedTruncatedJSON := html.EscapeString(truncatedJSON)
		responseMessage := fmt.Sprintf("<pre><code class=\"language-json\">%s</code></pre>", escapedTruncatedJSON)

		if bot.IsActive() && bot.Username != "" {
			botInputPeer, err := bgCtx.ResolveUsername(bot.Username)
			if err != nil {
				_ = fallbackHTMLMessage(&bgCtx, uChat.GetID(), uMsg.ID, escapedTruncatedJSON, pasteURL)
				return
			}

			chatInputPeer, err := bgCtx.ResolveInputPeerById(uChat.GetID())
			if err != nil {
				return
			}

			randomID := fmt.Sprintf("%d", rand.Int63())
			redisKey := fmt.Sprintf("json_paste:%s", randomID)

			payload := map[string]string{
				"text": responseMessage,
				"url":  pasteURL,
			}
			payloadBytes, _ := json.Marshal(payload)
			err = dbClient.Redis.Set(&bgCtx, redisKey, string(payloadBytes), 10*time.Minute).Err()
			if err != nil {
				_ = fallbackHTMLMessage(&bgCtx, uChat.GetID(), uMsg.ID, escapedTruncatedJSON, pasteURL)
				return
			}

			_ = bgCtx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

			results, err := bgCtx.Raw.MessagesGetInlineBotResults(&bgCtx, &tg.MessagesGetInlineBotResultsRequest{
				Bot:    botInputPeer.GetInputUser(),
				Peer:   chatInputPeer,
				Query:  fmt.Sprintf("json:%s", randomID),
				Offset: "",
			})
			if err != nil {
				_ = fallbackHTMLMessage(&bgCtx, uChat.GetID(), uMsg.ID, escapedTruncatedJSON, pasteURL)
				return
			}

			if len(results.Results) == 0 {
				_ = fallbackHTMLMessage(&bgCtx, uChat.GetID(), uMsg.ID, escapedTruncatedJSON, pasteURL)
				return
			}

			_, err = bgCtx.Raw.MessagesSendInlineBotResult(&bgCtx, &tg.MessagesSendInlineBotResultRequest{
				Peer:     chatInputPeer,
				RandomID: rand.Int63(),
				QueryID:  results.QueryID,
				ID:       results.Results[0].GetID(),
			})
			return
		}

		_ = fallbackHTMLMessage(&bgCtx, uChat.GetID(), uMsg.ID, escapedTruncatedJSON, pasteURL)
	}()

	return nil
}

func fallbackHTMLMessage(ctx *ext.Context, chatID int64, messageID int, escapedJSON string, pasteURL string) error {
	viewFullText := i18n.Localize("JsonViewFullJSON", nil, nil)
	response := fmt.Sprintf("<pre><code class=\"language-json\">%s</code></pre>\n\n🔗 <a href=\"%s\">%s</a>", escapedJSON, pasteURL, viewFullText)
	_, err := utils.EditMessageHTML(ctx, chatID, messageID, response)
	return err
}

func jsonInlineHandler(ctx context.Context, q *tg.UpdateBotInlineQuery) error {
	if !strings.HasPrefix(q.Query, "json:") {
		return manager.ErrNotMatched
	}

	parts := strings.Split(q.Query, ":")
	if len(parts) != 2 {
		return nil
	}
	id := parts[1]

	key := fmt.Sprintf("json_paste:%s", id)
	val, err := dbClient.Redis.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	var data struct {
		Text string `json:"text"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return err
	}

	buttons := [][]bot.Button{
		{
			{
				Text: i18n.Localize("JsonViewFullJSON", nil, nil),
				URL:  data.URL,
			},
		},
	}
	keyboard := bot.BuildInlineKeyboard(buttons)
	plainText, entities := utils.ParseHTML(data.Text)

	result := &tg.InputBotInlineResult{
		ID:   "json_result",
		Type: "article",
		SendMessage: &tg.InputBotInlineMessageText{
			Message:     plainText,
			Entities:    entities,
			ReplyMarkup: keyboard,
			NoWebpage:   true,
		},
	}
	result.SetTitle(i18n.Localize("JsonOutput", nil, nil))
	result.SetDescription(i18n.Localize("JsonOutputDesc", nil, nil))

	results := []tg.InputBotInlineResultClass{result}
	return bot.AnswerInlineQuery(ctx, q.QueryID, results)
}

func uploadToPasteRS(content string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Post("https://paste.rs/", "text/plain", strings.NewReader(content))
	if err != nil {
		return "", err
	}
	defer func() {
		err2 := resp.Body.Close()
		if err2 != nil {

		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	pasteURL := strings.TrimSpace(string(bodyBytes))
	if !strings.HasPrefix(pasteURL, "http") {
		return "", fmt.Errorf("invalid response URL from paste.rs: %s", pasteURL)
	}

	return pasteURL, nil
}
