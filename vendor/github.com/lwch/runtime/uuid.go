package runtime

import (
	"bytes"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// UUID generate uuid string with n
func UUID(n int, chars ...string) (string, error) {
	if len(chars) == 0 {
		chars = []string{"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-"}
	}
	charMap := chars[0]
	var ret bytes.Buffer
	for n > 0 {
		buf := make([]byte, n)
		readen, err := rand.Read(buf)
		if err != nil {
			return "", err
		}
		for _, ch := range buf[:readen] {
			if err := ret.WriteByte(charMap[int(ch)%len(charMap)]); err != nil {
				return "", err
			}
			n--
		}
	}
	return ret.String(), nil
}
