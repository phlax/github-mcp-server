package inventory

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstSentence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		cap  int
		want string
	}{
		{name: "empty", in: "", cap: 10, want: ""},
		{name: "single word", in: "hello", cap: 10, want: "hello"},
		{name: "newline wins", in: "first line\nsecond line", cap: 100, want: "first line"},
		{name: "period wins", in: "first sentence. second sentence", cap: 100, want: "first sentence"},
		{name: "cap wins", in: "abcdefghijklmnop", cap: 5, want: "abcde"},
		{name: "exact cap", in: "abcdefghij", cap: 10, want: "abcdefghij"},
		{name: "trims punctuation", in: "hello!!!", cap: 80, want: "hello"},
		{name: "multiline with punctuation", in: "hello.\nworld", cap: 80, want: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, firstSentence(tt.in, tt.cap))
		})
	}
}

func TestShortenToolDescription(t *testing.T) {
	t.Parallel()

	short := shortenToolDescription("Read issues. Includes useful metadata.")
	require.True(t, strings.HasSuffix(short, terseDescriptionHint))
	require.Contains(t, short, "Read issues")
	require.LessOrEqual(t, len(short), 160)
}

func TestShortenParamDescription(t *testing.T) {
	t.Parallel()

	in := "Repository owner. Use the organization or username that owns the repository."
	got := shortenParamDescription(in)
	require.Equal(t, "Repository owner", got)
	require.LessOrEqual(t, len(got), 80)
}
