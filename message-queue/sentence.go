package messageQueue

import (
	"strings"
)

type Sentence struct {
	Text      string
	Count     int // the amount of occurrences in the message queue
	WordCount int // the amount of words in the sentence
}

type SortedSentences []Sentence

func (ss SortedSentences) Len() int      { return len(ss) }
func (ss SortedSentences) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }

// Less orders the elements of SortedSentences by descending WordCount (1) and Count (2)
func (ss SortedSentences) Less(i, j int) bool {
	return ss[i].WordCount > ss[j].WordCount || ss[i].WordCount == ss[j].WordCount && ss[i].Count > ss[j].Count
}

// FindSimilarSentences returns a subarray of the sorted array containing only similar sentences to the one with originIndex
func (ss SortedSentences) FindSimilarSentences(originIndex int) SortedSentences {
	similarSbc := SortedSentences{}
	for i, sentence := range ss {
		if i != originIndex {
			if strings.ToLower(sentence.Text) == strings.ToLower(ss[originIndex].Text) {
				similarSbc = append(similarSbc, sentence)
			}
		}
	}
	return similarSbc
}

// TotalCount returns the sum of the counts of all the sentences in the array
func (ss SortedSentences) TotalCount() int {
	count := 0
	for _, sentence := range ss {
		count += sentence.Count
	}
	return count
}
