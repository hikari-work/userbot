package utils

import (
	"fmt"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func CheckAdminPermission(ctx *ext.Context, chatID int64, checkFunc func(rights tg.ChatAdminRights) bool) (bool, error) {
	inputPeer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		return false, fmt.Errorf("failed to resolve chat peer: %w", err)
	}

	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		resp, err := ctx.Raw.ChannelsGetParticipant(ctx, &tg.ChannelsGetParticipantRequest{
			Channel: &tg.InputChannel{
				ChannelID:  p.ChannelID,
				AccessHash: p.AccessHash,
			},
			Participant: &tg.InputPeerSelf{},
		})
		if err != nil {
			return false, fmt.Errorf("failed to get channel participant info: %w", err)
		}

		switch part := resp.Participant.(type) {
		case *tg.ChannelParticipantCreator:
			return true, nil
		case *tg.ChannelParticipantAdmin:
			return checkFunc(part.AdminRights), nil
		default:
			return false, nil
		}
	default:
		return true, nil
	}
}

func IsAdminOrSelf(ctx *ext.Context, chatID, userID int64) (bool, error) {
	if userID == ctx.Self.ID {
		return true, nil
	}

	inputPeer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		return false, err
	}

	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		inputUser, errUser := ctx.ResolveInputPeerById(userID)
		if errUser != nil {
			return false, errUser
		}
		pUser, ok := inputUser.(*tg.InputPeerUser)
		if !ok {
			return false, fmt.Errorf("invalid user peer")
		}

		resp, err := ctx.Raw.ChannelsGetParticipant(ctx, &tg.ChannelsGetParticipantRequest{
			Channel: &tg.InputChannel{
				ChannelID:  p.ChannelID,
				AccessHash: p.AccessHash,
			},
			Participant: &tg.InputPeerUser{
				UserID:     pUser.UserID,
				AccessHash: pUser.AccessHash,
			},
		})
		if err != nil {
			return false, err
		}

		switch resp.Participant.(type) {
		case *tg.ChannelParticipantCreator, *tg.ChannelParticipantAdmin:
			return true, nil
		default:
			return false, nil
		}
	default:
		return false, nil
	}
}
