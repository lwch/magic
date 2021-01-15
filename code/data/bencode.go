package data

const (
	request  = "q"
	response = "r"
	err      = "e"
)

type common struct {
	Transaction string `bencode:"t"`
	Type        string `bencode:"y"`
}

type query struct {
	Action string      `bencode:"q"`
	Data   interface{} `bencode:"a"`
}

func newCommon(t string) common {
	return common{
		Transaction: Rand(32),
		Type:        t,
	}
}

func newQuery(action string, data interface{}) query {
	return query{
		Action: action,
		Data:   data,
	}
}
