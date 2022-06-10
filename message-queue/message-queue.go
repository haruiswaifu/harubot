package messageQueue

import (
	"errors"
	twitchIrc "github.com/gempir/go-twitch-irc/v2"
	"harubot/emotes"
	"sort"
	"strings"
	"sync"
	"time"
)

func (mq *MessageQueue) Lock() {
	mq.lock.Lock()
}
func (mq *MessageQueue) Unlock() {
	mq.lock.Unlock()
}

type MessageQueue struct {
	queue       []twitchIrc.PrivateMessage
	lastMessage string
	lock        sync.Mutex
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		queue:       []twitchIrc.PrivateMessage{},
		lastMessage: "",
		lock:        sync.Mutex{},
	}
}

func NewMessageQueues(channels []string) map[string]*MessageQueue {
	r := map[string]*MessageQueue{}
	for _, c := range channels {
		r[c] = NewMessageQueue()
	}
	return r
}

const MQCapacity = 30

var thresholds = map[string][]float32{
	"jinnytty":      {8, 6, 5},
	"mizkif":        {12, 9, 8},
	"xqc":           {12, 9, 8},
	"erobb221":      {12, 9, 8},
	"trainwreckstv": {12, 9, 8},
	"zoil":          {12, 9, 8},
}
var defaultThresholds = []float32{10, 8, 7}

// pushes new element to end of queue
func (mq *MessageQueue) Push(m twitchIrc.PrivateMessage) {
	wordSplit := strings.Split(m.Message, " ")
	for _, w := range wordSplit {
		if strings.HasPrefix(w, "@") && len(w) > 2 {
			return // ignore messages with user mentions
		}
	}

	if len(mq.queue) == MQCapacity {
		mq.pop()
	}
	mq.queue = append(mq.queue, m)
}

// pops off first element
func (mq *MessageQueue) pop() {
	mq.queue = mq.queue[1:]
}

func (mq *MessageQueue) length() int {
	return len(mq.queue)
}

func (mq *MessageQueue) Clear() {
	mq.queue = []twitchIrc.PrivateMessage{}
}

// Velocity calculates the amount of messages per second of the messages in the queue
func (mq *MessageQueue) Velocity() float64 {
	mqSlice := mq.queue
	if len(mqSlice) == 0 {
		return 0
	}
	firstMessage, lastMessage := mqSlice[0], mqSlice[mq.length()-1]
	timeSpan := lastMessage.Time.Sub(firstMessage.Time)
	timeSpanSeconds := timeSpan.Seconds()
	amountOfMessages := mq.length()
	return float64(amountOfMessages) / timeSpanSeconds
}

// FindSpammedMessage finds the most spammed sentence (any in order combination of words) based on messages by unique users
func (mq *MessageQueue) FindSpammedMessage(channel string, emoteCache *emotes.Cache) (string, error) {
	initialLength := len(mq.queue)
	sentenceCountsByWordCount := map[int]map[string]int{}
	messageAuthors := map[string]map[string]bool{}
	sentenceTimes := map[string][]time.Time{}
	for _, v := range mq.queue {
		if strings.HasPrefix(v.Message, "!") {
			continue
		}
		words := strings.Split(v.Message, " ")
		nonEmptyWords := FilterEmptyWords(words)
		sentencePresenceByWordCount := map[int]map[string]bool{}
		currAuthor := v.User.ID
		currTime := v.Time

		if len(words) > 1 {
			// finds all sentences that are part of the message
			for i := range nonEmptyWords {
				for j := i + 1; j < len(nonEmptyWords); j++ {
					sentenceWords := nonEmptyWords[i : j+1]
					wordCount := j - i + 1
					if _, ok := sentencePresenceByWordCount[wordCount]; !ok {
						sentencePresenceByWordCount[wordCount] = map[string]bool{}
					}
					sentencePresenceByWordCount[wordCount][strings.Join(sentenceWords, " ")] = true
				}
			}
		} else {
			if _, ok := sentencePresenceByWordCount[1]; !ok {
				sentencePresenceByWordCount[1] = map[string]bool{}
			}
			sentencePresenceByWordCount[1][words[0]] = true
		}

		for wordCount, sentencePresence := range sentencePresenceByWordCount {
			for sentence := range sentencePresence {
				// author
				prevAuthors := messageAuthors[sentence]
				if prevAuthors == nil {
					messageAuthors[sentence] = map[string]bool{}
				}
				found := false
				for k := range prevAuthors {
					if k == currAuthor {
						found = true
					}
				}
				if found {
					continue
				}
				messageAuthors[sentence][currAuthor] = true

				// time
				prevTimes := sentenceTimes[sentence]
				if prevTimes == nil {
					sentenceTimes[sentence] = []time.Time{}
				}
				sentenceTimes[sentence] = append(sentenceTimes[sentence], currTime)

				// wordCount
				sentenceCountForWordCount := sentenceCountsByWordCount[wordCount]
				if sentenceCountForWordCount == nil {
					sentenceCountsByWordCount[wordCount] = map[string]int{}
				}
				sentenceCountsByWordCount[wordCount][sentence]++
			}
		}
	}

	sbc := SortedSentences{}
	for wordCount, sentenceCounts := range sentenceCountsByWordCount {
		for sentence, count := range sentenceCounts {
			sbc = append(sbc, Sentence{
				sentence,
				count,
				wordCount,
			})
		}
	}
	sort.Sort(sbc)

	for i, sentence := range sbc {
		wordCount := sentence.WordCount
		thresholdsIndex := wordCount - 1
		if thresholdsIndex > len(defaultThresholds)-1 {
			thresholdsIndex = len(defaultThresholds) - 1
		}
		similarSentences := sbc.FindSimilarSentences(i)
		countOfSimilarSentences := similarSentences.TotalCount()
		totalCount := countOfSimilarSentences + sentence.Count
		similarSentencesWithSelf := append(similarSentences, sentence)
		var sentenceVariantWithMaxOccurrences *Sentence
		for _, similarSentence := range similarSentencesWithSelf {
			if sentenceVariantWithMaxOccurrences == nil || similarSentence.Count > sentenceVariantWithMaxOccurrences.Count {
				sentenceVariantWithMaxOccurrences = &similarSentence
			}
		}

		threshs, found := thresholds[channel]
		if !found {
			threshs = defaultThresholds
		}
		if float32(totalCount) >= threshs[thresholdsIndex] {
			s := sentenceVariantWithMaxOccurrences.Text
			if !strings.HasPrefix(strings.ToLower(s), "!bet") && !strings.Contains(strings.ToLower(s), "residentsleeper") && !strings.Contains(strings.ToLower(s), "nigger") && s != "\U000e0000" {
				if len(mq.queue) >= initialLength { // check to ensure queue hasn't cleared since starting to find message
					if emoteCache.SentenceContainsEmotes(s, channel) {
						if mq.lastMessage == s {
							mq.lastMessage = s + " \U000e0000"
							return mq.lastMessage, nil
						}
						mq.lastMessage = s
						return s, nil
					}
				}
			}
		}
	}

	return "", errors.New("unable to find spammed message that meets requirements")
}
