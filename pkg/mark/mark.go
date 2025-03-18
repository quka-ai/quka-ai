package mark

import (
	"regexp"
	"strings"

	"github.com/quka-ai/quka-ai/pkg/utils"
)

type sensitiveWorker struct {
	contents []string
	index    map[string]string
}

var (
	HiddenRegexp = regexp.MustCompile(`\$hidden\[(.*?)\]`)
)

func ResolveHidden(text string, getValueFunc func(fakeValue string) string) (string, bool) {
	matches := HiddenRegexp.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		text = strings.Replace(text, match[0], getValueFunc(match[0]), 1)
	}
	return text, len(matches) > 0
}

func (s *sensitiveWorker) Do(text string) string {
	matches := HiddenRegexp.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		s.contents = append(s.contents, match[0])
		n := strings.ReplaceAll(match[0], match[1], utils.RandomStr(10))
		o := match[0]
		s.index[n] = o

		text = strings.Replace(text, o, n, 1)
	}
	return text
}

func (s *sensitiveWorker) Undo(text string) string {
	for n, o := range s.index {
		text = strings.ReplaceAll(text, n, o)
	}
	return text
}

func (s *sensitiveWorker) Map() map[string]string {
	return s.index
}

type sensitiveWords struct {
	Old string
	New string
}

func NewSensitiveWork() *sensitiveWorker {
	return &sensitiveWorker{
		index: make(map[string]string),
	}
}
