package dht

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestTable(t *testing.T) {
	table := newTable(nil, 8, 0)
	var id hashType
	for i := 0; i < 10000; i++ {
		rand.Read(id[:])
		table.add(newNode(nil, id, net.UDPAddr{}))
	}
	bk, height, cnt := printTable(table.root, "")
	fmt.Printf("max: %s %d %d\n", bk.String(), height, cnt)
}

func printTable(bk *bucket, dir string) (hashType, int, int) {
	if !bk.isLeaf() {
		prefix1, bits1, cnt1 := printTable(bk.leaf[0], "left")
		prefix2, bits2, cnt2 := printTable(bk.leaf[1], "right")
		if bits1 > bits2 {
			return prefix1, bits1, cnt1 + cnt2
		} else if bits2 > bits1 {
			return prefix2, bits2, cnt1 + cnt2
		} else if bytes.Compare(prefix1[:], prefix2[:]) < 0 {
			return prefix2, bits2, cnt1 + cnt2
		}
		return prefix1, bits1, cnt1 + cnt2
	}
	var ids []string
	for _, node := range bk.nodes {
		ids = append(ids, node.id.String())
	}
	fmt.Printf("%s: %s %d %s\n",
		bk.prefix.String(), dir, bk.bits, strings.Join(ids, ","))
	return bk.prefix, bk.bits, len(bk.nodes)
}
