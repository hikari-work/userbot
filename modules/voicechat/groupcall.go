package voicechat

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func getGroupCall(ctx *ext.Context, chatID int64) (*tg.InputGroupCall, error) {
	inputPeer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve chat peer: %w", err)
	}

	var call *tg.InputGroupCall

	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		fullChatResp, err := ctx.Raw.ChannelsGetFullChannel(ctx, &tg.InputChannel{
			ChannelID:  p.ChannelID,
			AccessHash: p.AccessHash,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get full channel: %w", err)
		}
		fullChannel, ok := fullChatResp.FullChat.(*tg.ChannelFull)
		if !ok {
			return nil, fmt.Errorf("failed to cast to ChannelFull")
		}

		if c, ok := fullChannel.GetCall(); ok {
			if igc, ok := c.(*tg.InputGroupCall); ok {
				call = &tg.InputGroupCall{
					ID:         igc.ID,
					AccessHash: igc.AccessHash,
				}
			}
		}

		if call == nil {
			_, err := ctx.Raw.PhoneCreateGroupCall(ctx, &tg.PhoneCreateGroupCallRequest{
				Peer:     p,
				RandomID: int(rand.Int31()),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create group call: %w", err)
			}

			time.Sleep(500 * time.Millisecond)

			fullChatResp, err = ctx.Raw.ChannelsGetFullChannel(ctx, &tg.InputChannel{
				ChannelID:  p.ChannelID,
				AccessHash: p.AccessHash,
			})
			if err == nil {
				if fullChannel, ok = fullChatResp.FullChat.(*tg.ChannelFull); ok {
					if c, ok := fullChannel.GetCall(); ok {
						if igc, ok := c.(*tg.InputGroupCall); ok {
							call = &tg.InputGroupCall{
								ID:         igc.ID,
								AccessHash: igc.AccessHash,
							}
						}
					}
				}
			}
		}

	case *tg.InputPeerChat:
		fullChatResp, err := ctx.Raw.MessagesGetFullChat(ctx, p.ChatID)
		if err != nil {
			return nil, fmt.Errorf("failed to get full chat: %w", err)
		}
		fullChat, ok := fullChatResp.FullChat.(*tg.ChatFull)
		if !ok {
			return nil, fmt.Errorf("failed to cast to ChatFull")
		}

		if c, ok := fullChat.GetCall(); ok {
			if igc, ok := c.(*tg.InputGroupCall); ok {
				call = &tg.InputGroupCall{
					ID:         igc.ID,
					AccessHash: igc.AccessHash,
				}
			}
		}

		if call == nil {
			_, err := ctx.Raw.PhoneCreateGroupCall(ctx, &tg.PhoneCreateGroupCallRequest{
				Peer:     p,
				RandomID: int(rand.Int31()),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create group call: %w", err)
			}

			// Wait a short moment for Telegram to propagate the new call
			time.Sleep(500 * time.Millisecond)

			fullChatResp, err = ctx.Raw.MessagesGetFullChat(ctx, p.ChatID)
			if err == nil {
				if fullChat, ok = fullChatResp.FullChat.(*tg.ChatFull); ok {
					if c, ok := fullChat.GetCall(); ok {
						if igc, ok := c.(*tg.InputGroupCall); ok {
							call = &tg.InputGroupCall{
								ID:         igc.ID,
								AccessHash: igc.AccessHash,
							}
						}
					}
				}
			}
		}
	}

	if call != nil {
		return call, nil
	}
	return nil, fmt.Errorf("no active voice chat found and could not start one in this chat")
}
