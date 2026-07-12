package download

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func getRandomID() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return n.Int64()
}

func parseLink(link string) (peer string, isPrivate bool, msgID int, err error) {
	link = strings.TrimSpace(link)

	webRegex := regexp.MustCompile(`(?:t\.me|telegram\.(?:me|dog))/(c/)?([a-zA-Z0-9_]+)/([0-9]+)`)
	if matches := webRegex.FindStringSubmatch(link); len(matches) == 4 {
		isPrivate = matches[1] == "c/"
		peer = matches[2]
		msgID, err = strconv.Atoi(matches[3])
		return peer, isPrivate, msgID, err
	}

	tgResolveRegex := regexp.MustCompile(`tg://resolve\?domain=([a-zA-Z0-9_]+)&post=([0-9]+)`)
	if matches := tgResolveRegex.FindStringSubmatch(link); len(matches) == 3 {
		peer = matches[1]
		msgID, err = strconv.Atoi(matches[2])
		return peer, false, msgID, err
	}

	tgPrivateRegex := regexp.MustCompile(`tg://privatepost\?channel=([0-9\-]+)&post=([0-9]+)`)
	if matches := tgPrivateRegex.FindStringSubmatch(link); len(matches) == 3 {
		peer = matches[1]
		msgID, err = strconv.Atoi(matches[2])
		return peer, true, msgID, err
	}

	return "", false, 0, fmt.Errorf("format link not acceptable: %s", link)
}

func resolvePeer(ctx *ext.Context, peer string, isPrivate bool) (int64, error) {
	if isPrivate {
		parsedChatID, err := strconv.ParseInt(peer, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("chat ID not valid: %w", err)
		}
		if parsedChatID > 0 {
			if parsedChatID < 1000000000000 {
				parsedChatID = -1000000000000 - parsedChatID
			} else {
				parsedChatID = -parsedChatID
			}
		}
		_, err = ctx.ResolveInputPeerById(parsedChatID)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve private chat ID %d: %w", parsedChatID, err)
		}
		return parsedChatID, nil
	}

	resolvedChat, err := ctx.ResolveUsername(peer)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve username '%s': %w", peer, err)
	}
	return resolvedChat.GetID(), nil
}

func getMessage(ctx *ext.Context, chatID int64, msgID int) (*tg.Message, error) {
	msgs, err := ctx.GetMessages(chatID, []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}})
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("message not found")
	}

	msg, ok := msgs[0].(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("message type not identified")
	}
	if msg.Media == nil {
		return nil, fmt.Errorf("message doesn't have media")
	}
	return msg, nil
}
