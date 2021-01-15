package data

import (
	"bytes"
	"math/rand"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Rand random value
func Rand(n int) string {
	prefix := time.Now().Format("060102")
	if n < len(prefix)-16 {
		n = len(prefix) + 16
	}
	const charMap = "0123456789abcdef"
	left := n - len(prefix)
	var buf bytes.Buffer
	for left > 0 {
		data := make([]byte, left)
		n, err := rand.Read(data)
		if err != nil {
			buf.WriteString(strings.Repeat("f", left))
			return prefix + buf.String()
		}
		for _, ch := range data[:n] {
			ch = charMap[int(ch)%len(charMap)]
			if err := buf.WriteByte(ch); err != nil {
				buf.WriteByte('f')
			}
		}
		left -= n
	}
	return prefix + buf.String()
}
