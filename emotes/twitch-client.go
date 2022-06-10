package emotes

import (
	"fmt"
	helix "github.com/nicklaw5/helix/v2"
	log "github.com/sirupsen/logrus"
)

type twitchClient struct {
	helix *helix.Client
}

type secrets struct {
	TwitchAccessToken string `json:"twitch-access-token"`
	TwitchClientID    string `json:"twitch-client-id"`
}

func newTwitchClient() (*twitchClient, error) {
	s, err := readSecrets()
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets: %s", err)
	}
	helixClient, err := helix.NewClient(&helix.Options{
		ClientID:        s.TwitchClientID,
		UserAccessToken: s.TwitchAccessToken,
	})
	if err != nil {
		return nil, err
	}
	return &twitchClient{
		helix: helixClient,
	}, nil
}

const (
	SubscriptionTier_NoSubscription = iota
	SubscriptionTier_1
	SubscriptionTier_2
	SubscriptionTier_3
)

func (t *twitchClient) checkSub(broadcasterId, userId string) int {
	resp, err := t.helix.CheckUserSubscription(&helix.UserSubscriptionsParams{
		BroadcasterID: broadcasterId,
		UserID:        userId,
	})
	if err != nil {
		log.Errorf("failed to check subscription to %s: %s", broadcasterId, err)
		return SubscriptionTier_NoSubscription
	}
	if resp.StatusCode != 200 {
		log.Errorf("failed to check subscription to %s: status code %d", broadcasterId, resp.StatusCode)
		return SubscriptionTier_NoSubscription
	}
	if len(resp.Data.UserSubscriptions) > 0 {
		return subscriptionTierStringToInt(resp.Data.UserSubscriptions[0].Tier)
	} else {
		return SubscriptionTier_NoSubscription
	}
}

func subscriptionTierStringToInt(subscriptionTierString string) int {
	switch subscriptionTierString {
	case "1000":
		return SubscriptionTier_1
	case "2000":
		return SubscriptionTier_2
	case "3000":
		return SubscriptionTier_3
	default:
		return SubscriptionTier_NoSubscription
	}
}

func (t *twitchClient) getChannelEmotes(broadcasterId string) ([]helix.Emote, error) {
	resp, err := t.helix.GetChannelEmotes(&helix.GetChannelEmotesParams{
		BroadcasterID: broadcasterId,
	})
	if err != nil {
		log.Errorf("failed to get twitch channel emotes for %s: %s", broadcasterId, err)
		return []helix.Emote{}, nil
	}
	return resp.Data.Emotes, nil
}
