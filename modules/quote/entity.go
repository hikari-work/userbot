package quote

type QuoteRequest struct {
	BackgroundColor string    `json:"backgroundColor"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	Scale           int       `json:"scale"`
	EmojiBrand      string    `json:"emojiBrand"`
	Messages        []Message `json:"messages"`
}

type Message struct {
	From         User          `json:"from"`
	Text         string        `json:"text"`
	Entities     []Entity      `json:"entities"`
	Avatar       bool          `json:"avatar"`
	ReplyMessage *ReplyMessage `json:"replyMessage,omitempty"`
	Media        *MessageMedia `json:"media,omitempty"`
}

type MessageMedia struct {
	Url string `json:"url,omitempty"`
}

type User struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Photo     Photo  `json:"photo"`
}

type Photo struct {
	BigFileId string `json:"big_file_id,omitempty"`
	Url       string `json:"url,omitempty"`
}

type Entity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type ReplyMessage struct {
	Name     string        `json:"name"`
	Text     string        `json:"text"`
	Entities []interface{} `json:"entities"`
	ChatId   int           `json:"chatId"`
	From     ReplyFrom     `json:"from"`
}

type ReplyFrom struct {
	Id    int        `json:"id"`
	Name  string     `json:"name"`
	Photo ReplyPhoto `json:"photo"`
}

type ReplyPhoto struct {
	Url string `json:"url"`
}
