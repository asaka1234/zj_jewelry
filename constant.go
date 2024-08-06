package zj_jewelry

const (
	WidgetDataWssAddress = "ws://159.75.182.253:9502/socket.io/?EIO=3&transport=websocket"
)

var LegalSymbolMap = map[string]string{

	//国内行情
	"AU9999":   "黄金9999",
	"AUTD":     "黄金T+D",
	"AGTD":     "白银T+D",
	"TMAP9995": "铂金9995",

	//国际行情
	"XAU":    "伦敦金",
	"XAG":    "伦敦银",
	"XAP":    "伦敦铂",
	"XPD":    "伦敦钯",
	"GLNC":   "美黄金",
	"SLNC":   "美白银",
	"PANC":   "美钯金",
	"PLNC":   "美铂金",
	"USDCNH": "美元",

	//现货价格
	"TMAU": "公斤条",
	"SBJ":  "水贝金",
	"TMAP": "铂金",
	"TMAG": "白银",
	"TMPD": "钯金",

	/*
		//这些貌似没展示/没用到
		"AUKB9999": "公斤条", //TODO 要用哪一个的价格呢?
		"AP950":   "铂金950",
		"AP990":   "钯金990",
		"AU9985":  "千足金",
		"18K":     "18K旧料",
		"USRd":    "美铑金",
		"Rhodium": "铑金",
	*/
}
