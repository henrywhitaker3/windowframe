package test

import (
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

func init() {
	_ = gofakeit.Seed(0)
}

func Word() string {
	return gofakeit.Word()
}

func Sentence(words int) string {
	out := []string{}
	for len(out) < words {
		tmp := strings.Split(gofakeit.Sentence(), " ")
		i := 0
		for len(out) < words {
			if len(tmp) <= i {
				break
			}
			out = append(out, tmp[i])
		}
	}
	return strings.Join(out, " ")
}

func Email() string {
	return gofakeit.Email()
}

func Letters(length int) string {
	return gofakeit.LetterN(uint(length))
}

func Name() string {
	return gofakeit.Name()
}
