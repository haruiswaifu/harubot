package emotes

import (
	"encoding/json"
	"fmt"
	"github.com/forPelevin/gomoji"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SelfUserId = "488987844"
)

type Cache struct {
	emotesByChannel map[string]map[string]bool // cached
	globalEmotes    map[string]bool            // cached
	channelIds      map[string]string          // cached
	channels        []string                   // passed in
}

func NewCache(channels []string) *Cache {
	ebc := map[string]map[string]bool{}
	for _, channel := range channels {
		ebc[channel] = map[string]bool{}
	}
	newCache := &Cache{
		channels:        channels,
		emotesByChannel: ebc,
		globalEmotes:    map[string]bool{},
		channelIds:      map[string]string{},
	}
	newCache.fetchChannelIDs(channels)
	return newCache
}

func (c *Cache) clear() {
	for channel := range c.emotesByChannel {
		c.emotesByChannel[channel] = map[string]bool{}
	}
	c.globalEmotes = map[string]bool{}
}

const (
	emotesAPIEndpoint         = "https://emotes.adamcy.pl"
	emotesAPIVersion          = "v1"
	allServicesRegex          = "7tv.bttv.ffz.twitch"
	allServicesButTwitchRegex = "7tv.bttv.ffz"
)

func emotesAPIBaseURL() string {
	return fmt.Sprintf("%s/%s", emotesAPIEndpoint, emotesAPIVersion)
}

func (c *Cache) fetchEmotes() {
	c.fetchGlobalEmotes()
	c.fetchChannelEmotes(c.channels)
}

func doGetRequestAndRead(url string, headers map[string]string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("received non-OK status code: %d %s", resp.StatusCode, resp.Status)
		return nil, err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

type getChannelIDResponse struct {
	ID int `json:"id"`
}

func (c *Cache) getChannelID(channel string) (*getChannelIDResponse, error) {
	bodyBytes, err := doGetRequestAndRead(fmt.Sprintf("%s/channel/%s/id", emotesAPIBaseURL(), channel), map[string]string{})
	if err != nil {
		return nil, err
	}
	responseStruct := getChannelIDResponse{}
	err = json.Unmarshal(bodyBytes, &responseStruct)
	if err != nil {
		return nil, err
	}
	return &responseStruct, nil
}

func (c *Cache) fetchChannelIDs(channels []string) {
	failedAtLeastOnce := false
	for _, channel := range channels {
		getChannelIDResp, err := c.getChannelID(channel)
		if err != nil {
			failedAtLeastOnce = true
			log.Errorf("failed to get channel ID for %s: %s", channel, err)
			continue
		}
		c.channelIds[channel] = strconv.Itoa(getChannelIDResp.ID)
	}
	if failedAtLeastOnce {
		channelIdsBytes, err := ioutil.ReadFile("./channel-ids.json")
		if err != nil {
			log.Errorf("failed to read channel ids from file: %s", err)
		}
		err = json.Unmarshal(channelIdsBytes, &c.channelIds)
		if err != nil {
			log.Errorf("failed to unmarshal channel ids from file: %s", err)
		}
	}
	marshalledChannelIds, err := json.Marshal(&c.channelIds)
	if err != nil {
		log.Errorf("failed to marshal channel ids: %s", err)
	}
	err = ioutil.WriteFile("./channel-ids.json", marshalledChannelIds, 0644)
	if err != nil {
		log.Errorf("failed to write channel ids: %s", err)
	}
}

type getGlobalEmotesResponse []getGlobalEmotesEmote
type getGlobalEmotesEmote struct {
	Code string `json:"code"`
}

func (c *Cache) getGlobalEmotes() (*getGlobalEmotesResponse, error) {
	bodyBytes, err := doGetRequestAndRead(fmt.Sprintf("%s/global/emotes/%s", emotesAPIBaseURL(), allServicesRegex), map[string]string{})
	if err != nil {
		return nil, err
	}
	responseStruct := getGlobalEmotesResponse{}
	err = json.Unmarshal(bodyBytes, &responseStruct)
	if err != nil {
		return nil, err
	}
	return &responseStruct, nil
}

func (c *Cache) fetchGlobalEmotes() {
	globalEmotes, err := c.getGlobalEmotes()
	if err != nil {
		log.Errorf("failed to get global emotes: %s", err)
		return
	}
	for _, globalEmote := range *globalEmotes {
		c.globalEmotes[globalEmote.Code] = true
	}

	tc, err := newTwitchClient()
	if err != nil {
		log.Errorf("failed to create twitch client: %s", err)
	}
	for _, channelId := range c.channelIds {
		if subscriptionTier := checkSub(channelId, SelfUserId); subscriptionTier != SubscriptionTier_NoSubscription {
			emotes, err := tc.getChannelEmotes(channelId)
			if err != nil {
				log.Errorf("failed to get twitch emotes for channel %s: %s", channelId, err)
				continue
			}
			for _, emote := range emotes {
				if emote.EmoteType != "subscriptions" {
					continue
				}
				emoteSubscriptionTier := subscriptionTierStringToInt(emote.Tier)
				if emoteSubscriptionTier <= subscriptionTier {
					c.globalEmotes[emote.Name] = true
				}
			}
			time.Sleep(5 * time.Second) // avoid rate limits
		}
	}
}

func readSecrets() (*secrets, error) {
	secretsBytes, err := ioutil.ReadFile("./secrets.json")
	if err != nil {
		return nil, err
	}
	s := &secrets{}
	err = json.Unmarshal(secretsBytes, s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

type getChannelEmotesResponse getGlobalEmotesResponse // currently same structure

func (c *Cache) getChannelEmotes(channelID string, servicesRegex string) (*getChannelEmotesResponse, error) {
	bodyBytes, err := doGetRequestAndRead(fmt.Sprintf("%s/channel/%s/emotes/%s", emotesAPIBaseURL(), channelID, servicesRegex), map[string]string{})
	if err != nil {
		return nil, err
	}
	responseStruct := getChannelEmotesResponse{}
	err = json.Unmarshal(bodyBytes, &responseStruct)
	if err != nil {
		return nil, err
	}
	return &responseStruct, nil
}

func (c *Cache) fetchChannelEmotes(channels []string) {
	for _, channel := range channels {
		channelID, ok := c.channelIds[channel]
		if !ok {
			log.Errorf("failed to get channel emotes for %s: failed to find channel id in cache", channel)
			continue
		}
		channelEmotes, err := c.getChannelEmotes(channelID, allServicesButTwitchRegex)
		if err != nil {
			log.Errorf("failed to get channel emotes for %s: %s", channel, err)
			continue
		}
		for _, channelEmote := range *channelEmotes {
			c.emotesByChannel[channel][channelEmote.Code] = true
		}
		time.Sleep(350 * time.Millisecond) // avoid rate limits
	}
}

func (c *Cache) RoutinelyRefreshCache(interval int) {
	time.Sleep(1 * time.Minute)
	for {
		c.clear()
		c.fetchEmotes()
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}

func (c *Cache) IsWordAnEmoteInChannel(word string, channel string) bool {
	if gomoji.ContainsEmoji(word) {
		return true
	}
	for globalEmote := range c.globalEmotes {
		if word == globalEmote {
			return true
		}
	}
	for channelEmote := range c.emotesByChannel[channel] {
		if word == channelEmote {
			return true
		}
	}
	return false
}

func (c *Cache) MessageWithOnlyEmotes(message string, channel string) string {
	words := strings.Split(message, " ")
	emotes := []string{}
	for _, word := range words {
		if c.IsWordAnEmoteInChannel(word, channel) {
			emotes = append(emotes, word)
		}
	}
	asString := strings.Join(emotes, " ")
	return asString
}

func (c *Cache) MessageContainsOnlyEmotes(message string, channel string) bool {
	words := strings.Split(message, " ")
	for _, word := range words {
		if !c.IsWordAnEmoteInChannel(word, channel) {
			return false
		}
	}
	return true
}

func (c *Cache) SentenceContainsEmotes(sentence string, channel string) bool {
	for _, w := range strings.Split(sentence, " ") {
		if c.IsWordAnEmoteInChannel(w, channel) {
			return true
		}
	}
	return false
}
