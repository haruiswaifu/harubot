package main

import (
	"encoding/json"
	"fmt"
	twitchIrc "github.com/gempir/go-twitch-irc/v2"
	log "github.com/sirupsen/logrus"
	colorState "harubot/color-state"
	"harubot/emotes"
	messageQueue "harubot/message-queue"
	personalMessageQueue "harubot/personal-message-queue"
	renewUserToken "harubot/renew-user-token"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Secrets struct {
	Username           string `json:"username"`
	OauthKey           string `json:"oauth-key"`
	TwitchAccessToken  string `json:"twitch-access-token"`
	TwitchClientID     string `json:"twitch-client-id"`
	TwitchClientSecret string `json:"twitch-client-secret"`
	TwitchRefreshToken string `json:"twitch-refresh-token"`
}

type environmentVariables struct {
	MinimumChatVelocity              float64  `json:"minimum-chat-velocity"`
	Channels                         []string `json:"channels"`
	SelfUsername                     string   `json:"self-username"`
	SelfDisplayname                  string   `json:"self-displayname"`
	SelfUserId                       string   `json:"self-user-id"`
	EmoteCacheRefreshIntervalMinutes int      `json:"emote-cache-refresh-interval-minutes"`
	Colors                           []string `json:"colors"`
	PersonalMessageQueueCapacity     int      `json:"personal-message-queue-capacity"`
	TokenRefreshIntervalHours        int      `json:"token-refresh-interval-hours"`
}

func joinChannels(client *twitchIrc.Client, channels []string) {
	for _, c := range channels {
		client.Join(c)
		log.Printf("joined channel #%s", c)
		time.Sleep(2 * time.Second) // avoid rate limits
	}
}

type state struct {
	messageQueuesByChannel map[string]*messageQueue.MessageQueue
	personalMessageQueue   *personalMessageQueue.PersonalMessageQueue
	client                 *twitchIrc.Client
	emoteCache             *emotes.Cache
	colorState             *colorState.ColorState
	autoReplyTimes         map[string]time.Time
	minimumChatVelocity    float64
	selfUsername           string
	selfDisplayname        string
	connected              bool
}

func newState(s *Secrets, envVars *environmentVariables) *state {
	emoteCache := emotes.NewCache(envVars.Channels, envVars.SelfUserId)
	go emoteCache.RoutinelyRefreshCache(envVars.EmoteCacheRefreshIntervalMinutes)

	client := twitchIrc.NewClient(s.Username, s.OauthKey)
	go joinChannels(client, envVars.Channels)

	pmq := personalMessageQueue.NewPersonalMessageQueue(envVars.PersonalMessageQueueCapacity)

	cs := colorState.NewColorState(envVars.Colors)
	go cs.RoutinelyChangeColor(client, envVars.SelfUsername, pmq)

	mqs := messageQueue.NewMessageQueues(envVars.Channels)
	autoReplyTimes := map[string]time.Time{}

	go renewUserToken.RoutinelyRefreshToken(time.Duration(envVars.TokenRefreshIntervalHours) * time.Hour)

	return &state{
		emoteCache:             emoteCache,
		colorState:             cs,
		client:                 client,
		personalMessageQueue:   pmq,
		autoReplyTimes:         autoReplyTimes,
		messageQueuesByChannel: mqs,
		minimumChatVelocity:    envVars.MinimumChatVelocity,
		selfUsername:           envVars.SelfUsername,
		selfDisplayname:        envVars.SelfDisplayname,
		connected:              false,
	}
}

func readJSON(path string, structure any) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bytes, structure)
	if err != nil {
		return err
	}
	return nil
}

func setup() {
	s := &Secrets{}
	err := readJSON("./secrets.json", s)
	if err != nil {
		log.Fatalf("failed to read secrets: %s", err)
	}
	c := map[string]string{}
	err = readJSON("./channel-ids.json", &c)
	if err != nil {
		log.Fatalf("failed to read channel ids: %s", err)
	}
	e := &environmentVariables{}
	err = readJSON("./env.json", e)
	if err != nil {
		log.Fatalf("failed to read environment variables: %s", err)
	}

	state := newState(s, e)

	state.client.OnReconnectMessage(func(m twitchIrc.ReconnectMessage) {
		log.Println("received RECONNECT")
	})
	state.client.OnPingMessage(func(m twitchIrc.PingMessage) {
		log.WithFields(log.Fields{
			"message": m.Message,
		}).Info("received PING")
	})
	state.client.OnPongMessage(func(m twitchIrc.PongMessage) {
		log.WithFields(log.Fields{
			"message": m.Message,
		}).Info("received PONG")
	})
	state.client.OnNoticeMessage(func(m twitchIrc.NoticeMessage) {
		log.WithFields(log.Fields{
			"channel": m.Channel,
			"message": m.Message,
		}).Info("received NOTICE")
	})

	state.client.OnConnect(func() {
		if state.connected == true {
			log.Fatalf("restarting because of reconnect")
		}
		log.Infoln("connected")
		state.connected = true
	})

	state.client.OnPrivateMessage(func(m twitchIrc.PrivateMessage) {
		state.makePyramids(m)
		state.autoReply(m)
		state.onSelfMessage(m)
		state.spamBot(m)
	})

	err = state.client.Connect()
	if err != nil {
		log.Fatalf("failed to connect: %s", err)
	}
}

func main() {
	setup()
}

func (state *state) onSelfMessage(m twitchIrc.PrivateMessage) {
	mq := state.messageQueuesByChannel[m.Channel]
	if strings.ToLower(m.User.Name) == state.selfUsername {
		mq.Clear()
		state.personalMessageQueue.Push(time.Now())
	}
}

func (state *state) say(channel string, message string) {
	if state.personalMessageQueue.Velocity() > 0.66 {
		log.WithFields(log.Fields{
			"personal-message-velocity": fmt.Sprintf("%f messages/second", state.personalMessageQueue.Velocity()),
		}).Info("not sending message because personal rate limit was hit")
		return // twitch global rate limit of 20 messages per 30 seconds
	}
	state.client.Say(channel, message)
	state.personalMessageQueue.Push(time.Now())
}

func (state *state) spamBot(m twitchIrc.PrivateMessage) {
	mq := state.messageQueuesByChannel[m.Channel]
	words := strings.SplitN(m.Message, " ", 6)
	if len(words) == 6 {
		return // don't add long messages to queue for perf reasons
	}
	mq.Push(m)
	if mq.Velocity() < state.minimumChatVelocity {
		return // don't try to echo spammed messages in slow chat
	}
	spammedMessage, err := mq.FindSpammedMessage(m.Channel, state.emoteCache)
	if err == nil {
		state.say(m.Channel, spammedMessage)
		log.WithFields(log.Fields{
			"channel": m.Channel,
			"message": spammedMessage,
		}).Info("echoed spammed message")
		mq.Clear()
	}
}

func (state *state) doesNotMentionOthers(m twitchIrc.PrivateMessage, usersInChannel []string) bool {
	if len(usersInChannel) == 0 {
		return false
	}

	for _, userInChannel := range usersInChannel {
		if strings.ToLower(userInChannel) == state.selfUsername {
			continue
		}
		if strings.Contains(strings.ToLower(m.Message), strings.ToLower(userInChannel)) {
			return false
		}
	}
	return true
}

func (state *state) autoReply(m twitchIrc.PrivateMessage) {
	cooldown := 30 * time.Second
	lastReplyTime, lastReplyTimeFound := state.autoReplyTimes[m.User.Name]

	containsMyName := strings.Contains(strings.ToLower(m.Message), state.selfUsername) ||
		state.selfDisplayname != "" && strings.Contains(strings.ToLower(m.Message), state.selfDisplayname)
	isFromMe := strings.ToLower(m.User.Name) == state.selfUsername
	isNotOnCooldown := !lastReplyTimeFound || time.Since(lastReplyTime) > cooldown
	isFromMod, _ := m.User.Badges["moderator"]
	isFromStaff, _ := m.User.Badges["staff"]
	isFromAdmin, _ := m.User.Badges["admin"]
	isFromScaryPerson := isFromMod == 1 || isFromStaff == 1 || isFromAdmin == 1

	if containsMyName && !isFromMe && isNotOnCooldown && !isFromScaryPerson {
		go state.sendAutoReply(m)
	}
}

func (state *state) sendAutoReply(m twitchIrc.PrivateMessage) {
	time.Sleep(time.Duration((rand.Float32()*10)+2) * time.Second)

	usersInChannel, err := state.client.Userlist(m.Channel)
	if err != nil {
		log.Infof("failed to get userlist: %s\n", err)
		return
	}
	if !state.doesNotMentionOthers(m, usersInChannel) {
		log.WithFields(log.Fields{
			"channel": m.Channel,
			"user":    m.User.Name,
		}).Info("not autoreplying because message mentions others")
		return
	}

	maxEmotes := 24
	emotesToReply := state.emoteCache.MessageWithOnlyEmotes(m.Message, m.Channel)
	cappedEmotes := []string{}
	splitEmotes := strings.SplitN(emotesToReply, " ", maxEmotes+1)
	for i, emote := range splitEmotes {
		if i == maxEmotes {
			break
		}
		cappedEmotes = append(cappedEmotes, emote)
	}
	emotesToReplyCapped := strings.Join(cappedEmotes, " ")

	replyMessage := fmt.Sprintf("@%s, %s", m.User.DisplayName, emotesToReplyCapped)
	if emotesToReplyCapped != "" {
		state.autoReplyTimes[m.User.Name] = time.Now()
		state.say(m.Channel, replyMessage)
		log.WithFields(log.Fields{
			"channel":       m.Channel,
			"user":          m.User.Name,
			"reply-message": replyMessage,
		}).Info("autoreplied")
	}
}

func (state *state) makePyramids(m twitchIrc.PrivateMessage) {
	if m.User.Name == state.selfUsername && strings.HasPrefix(m.Message, "!pyramid") {
		args := strings.Split(m.Message, " ")
		if len(args) < 4 {
			return
		}
		lastI := len(args) - 1
		atomicMessage := ""
		for i := 1; i < lastI-1; i++ {
			if i != 1 {
				atomicMessage += " "
			}
			atomicMessage += args[i]
		}
		size, err2 := strconv.Atoi(args[lastI-1])
		if err2 != nil {
			return
		}
		delay, err3 := strconv.Atoi(args[lastI])
		if err3 != nil {
			return
		}
		for i := 0; i < size; i++ {
			message := ""
			for j := 1; j <= i+1; j++ {
				if j != 1 {
					message += " "
				}
				message += atomicMessage
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
			state.say(m.Channel, message)
		}
		for i := size - 2; i >= 0; i-- {
			message := ""
			for j := 1; j <= i+1; j++ {
				if j != 1 {
					message += " "
				}
				message += atomicMessage
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
			state.say(m.Channel, message)
		}
		log.WithFields(log.Fields{
			"atomic-message": atomicMessage,
			"size":           size,
			"delay":          fmt.Sprintf("%dms", delay),
			"channel":        m.Channel,
		}).Info("made pyramid")
	}
}
