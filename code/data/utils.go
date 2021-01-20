package data

import (
	"bytes"
	"math/rand"
	"strings"
	"time"
)

const ver = "1000"

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

// RandID random id: http://www.bittorrent.org/beps/bep_0020.html
func RandID() [20]byte {
	const charMap = "0123456789abcdef"
	var id [20]byte
	id[0] = '-'
	id[1] = 'M'
	id[2] = 'G'
	copy(id[3:], ver)
	id[7] = '-'
	for i := 8; i < 20; {
		n, err := rand.Read(id[i:])
		if err != nil {
			for j := i; j < 20; j++ {
				id[j] = 'f'
			}
			return id
		}
		for j := i; j < i+n; j++ {
			id[j] = charMap[int(id[j])%len(charMap)]
		}
		i += n
	}
	return id
}
