package zj_jewelry

// SocketInterface ...
type SocketInterface interface {
	//AddSymbols(symbols []interface{}) error //symbol不需要添加来源exchange
	//RemoveSymbols(symbols []interface{}) error
	Init() error
	Close() error
}

type InitMessage struct {
	Sid          string `json:"sid"`
	PingInterval int    `json:"pingInterval"`
	PingTimeout  int    `json:"pingTimeout"`
}

type QuoteMessage struct {
	Sid       map[string]SymbolQuote `mapstructure:"items" json:"items"`
	PubTime   string                 `mapstructure:"pubtime" json:"pubtime"`
	Result    string                 `mapstructure:"result" json:"result"`
	SbjStatus bool                   `mapstructure:"sbj_status" json:"sbj_status"`
}

// 单个symbol的价格
type SymbolQuote struct {
	Code *string `mapstructure:"C" json:"C"`
	Name *string `mapstructure:"name" json:"name"`
	Low  *string `mapstructure:"L" json:"L"` //价格没有的话就是 "-"
	High *string `mapstructure:"H" json:"H"`
	Sell *string `mapstructure:"Sell" json:"Sell"`
	Buy  *string `mapstructure:"Buy" json:"Buy"`
}

// -------------------------------------------------------
type Ticker struct {
	Symbol string `json:"symbol"`
	Ask    string `json:"ask"`
	Bid    string `json:"bid"`
	High   string `json:"high"`
	Low    string `json:"low"` //价格没有的话就是 "-"
}

// Flags ...
type Flags struct {
	Flags []string `json:"flags"`
}

// OnReceiveDataCallback ...
type OnReceiveDataCallback func(data []Ticker)

// OnErrorCallback ...
type OnErrorCallback func(err error, context string)
