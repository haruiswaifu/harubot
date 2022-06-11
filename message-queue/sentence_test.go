package messagequeue

import (
	"reflect"
	"testing"
)

func TestSortedSentences_FindSimilarSentences(t *testing.T) {
	type args struct {
		originIndex int
	}
	tests := []struct {
		name string
		ss   SortedSentences
		args args
		want SortedSentences
	}{
		{
			name: "basic test case",
			ss: []Sentence{{
				Text: "something",
			}, {
				Text: "Something",
			}},
			args: args{
				originIndex: 0,
			},
			want: []Sentence{{
				Text: "Something",
			}},
		},
		{
			name: "basic fail case",
			ss: []Sentence{{
				Text: "blabla",
			}, {
				Text: "something else entirely",
			}},
			args: args{
				originIndex: 0,
			},
			want: []Sentence{}, // expect no similar sentences
		},
		//{
		//	name: "same messages with a lot of typos",
		//	ss: []Sentence{{
		//		Text: "PepeLaugh he doesn't know",
		//	}, {
		//		Text: "PepLaugh he doesnt know",
		//	}},
		//	args: args{
		//		originIndex: 0,
		//	},
		//	want: []Sentence{{
		//		Text: "PepLaugh he doesnt know",
		//	}},
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ss.FindSimilarSentences(tt.args.originIndex); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindSimilarSentences() = %v, want %v", got, tt.want)
			}
		})
	}
}
