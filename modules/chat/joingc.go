package chat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "JoinGC",
		Description: "Join channels or groups from Telegram links",
		Commands:    []string{"joingc", "join"},
		OnlyOut:     true,
		Help:        helpJoinGC,
		Handler:     joinGCHandler,
	})
}

func helpJoinGC() string {
	return "Format:\n" +
		"<code>.joingc</code> or <code>.join</code> - Reply to a message with Telegram link(s) to join all of them"
}

type ParsedLink struct {
	Raw    string
	IsHash bool
	Target string
}

func extractTelegramLinks(text string) []ParsedLink {
	var results []ParsedLink
	seen := make(map[string]bool)

	tgSchemeRegex := regexp.MustCompile(`(?i)tg://join\?invite=([a-zA-Z0-9_\-]+)`)
	for _, match := range tgSchemeRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			hash := match[1]
			key := "hash:" + hash
			if !seen[key] {
				seen[key] = true
				results = append(results, ParsedLink{
					Raw:    match[0],
					IsHash: true,
					Target: hash,
				})
			}
		}
	}

	tmeRegex := regexp.MustCompile(`(?i)(?:https?://)?(?:t\.me|telegram\.me)/(?:\+([a-zA-Z0-9_\-]+)|joinchat/([a-zA-Z0-9_\-]+)|([a-zA-Z0-9_]{3,}))`)
	for _, match := range tmeRegex.FindAllStringSubmatch(text, -1) {
		raw := match[0]
		if match[1] != "" {
			hash := match[1]
			key := "hash:" + hash
			if !seen[key] {
				seen[key] = true
				results = append(results, ParsedLink{
					Raw:    raw,
					IsHash: true,
					Target: hash,
				})
			}
		} else if match[2] != "" {
			hash := match[2]
			key := "hash:" + hash
			if !seen[key] {
				seen[key] = true
				results = append(results, ParsedLink{
					Raw:    raw,
					IsHash: true,
					Target: hash,
				})
			}
		} else if match[3] != "" {
			username := match[3]
			key := "user:" + strings.ToLower(username)
			if !seen[key] {
				seen[key] = true
				results = append(results, ParsedLink{
					Raw:    raw,
					IsHash: false,
					Target: username,
				})
			}
		}
	}

	return results
}

func joinGCHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uMsg == nil || uChat == nil {
		return nil
	}

	replyHeader, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || replyHeader.ReplyToMsgID == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("JoinGCReplyRequired", nil, nil))
		return nil
	}

	msgs, err := ctx.GetMessages(uChat.GetID(), []tg.InputMessageClass{&tg.InputMessageID{ID: replyHeader.ReplyToMsgID}})
	if err != nil || len(msgs) == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("JoinGCReplyRequired", nil, nil))
		return nil
	}

	repliedMsg, ok := msgs[0].(*tg.Message)
	if !ok {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("JoinGCReplyRequired", nil, nil))
		return nil
	}

	links := extractTelegramLinks(repliedMsg.Message)
	if len(links) == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("JoinGCNoLinksFound", nil, nil))
		return nil
	}

	var reportLines []string

	for i, link := range links {
		procText := i18n.Localize("JoinGCProcessing", map[string]interface{}{
			"Current": i + 1,
			"Total":   len(links),
		}, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, procText)

		var joinErr error
		if link.IsHash {
			_, joinErr = ctx.Raw.MessagesImportChatInvite(ctx, link.Target)
		} else {
			resolvedChat, resolveErr := ctx.ResolveUsername(link.Target)
			if resolveErr != nil {
				joinErr = resolveErr
			} else {
				inputPeer, pErr := ctx.ResolveInputPeerById(resolvedChat.GetID())
				if pErr != nil {
					joinErr = pErr
				} else if pChannel, ok := inputPeer.(*tg.InputPeerChannel); ok {
					inputChannel := &tg.InputChannel{
						ChannelID:  pChannel.ChannelID,
						AccessHash: pChannel.AccessHash,
					}
					_, joinErr = ctx.Raw.ChannelsJoinChannel(ctx, inputChannel)
				} else {
					joinErr = fmt.Errorf("peer is not a channel/supergroup")
				}
			}
		}

		if joinErr == nil {
			reportLines = append(reportLines, fmt.Sprintf("✅ <b>%s</b>: Joined", link.Raw))
		} else {
			errStr := joinErr.Error()
			upperErr := strings.ToUpper(errStr)

			if strings.Contains(upperErr, "USER_ALREADY_PARTICIPANT") {
				alreadyText := i18n.Localize("JoinGCAlreadyMember", nil, nil)
				reportLines = append(reportLines, fmt.Sprintf("✅ <b>%s</b>: %s", link.Raw, alreadyText))
			} else if strings.Contains(upperErr, "INVITE_REQUEST_SENT") {
				reqSentText := i18n.Localize("JoinGCRequestSent", nil, nil)
				reportLines = append(reportLines, fmt.Sprintf("✅ <b>%s</b>: %s", link.Raw, reqSentText))
			} else {
				reportLines = append(reportLines, fmt.Sprintf("❌ <b>%s</b>: %s", link.Raw, errStr))
			}
		}
	}

	reportTitle := i18n.Localize("JoinGCReportTitle", map[string]interface{}{"Total": len(links)}, nil)
	finalMsg := reportTitle + "\n\n" + strings.Join(reportLines, "\n")
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, finalMsg)

	return nil
}
