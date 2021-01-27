package dht

type initData struct {
	tx string
	n  *node
}

type initQueue struct {
	data []initData
	idx  int
}

func newInitQueue(max int) *initQueue {
	return &initQueue{data: make([]initData, max)}
}

func (q *initQueue) push(tx string, n *node) {
	q.data[q.idx%len(q.data)] = initData{
		tx: tx,
		n:  n,
	}
	q.idx++
}

func (q *initQueue) find(tx string) *node {
	n := len(q.data)
	if q.idx < len(q.data) {
		n = q.idx
	}
	for i := 0; i < n; i++ {
		node := q.data[i]
		if node.tx == tx {
			return node.n
		}
	}
	return nil
}

func (q *initQueue) unset(tx string) {
	n := len(q.data)
	if q.idx < len(q.data) {
		n = q.idx
	}
	for i := 0; i < n; i++ {
		node := &q.data[i]
		if node.tx == tx {
			node.tx = ""
			node.n = nil
			return
		}
	}
}
