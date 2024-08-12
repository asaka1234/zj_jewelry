package zj_jewelry

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

var LegalSymbolList = []string{

	//现货价格
	"Unknow-1", //黄金
	//"SBJ":  "水贝金",
	"TMAG", //白银",
	"TMAP", //: "铂金",
	"TMPD", //: "钯金",

	//国内行情
	"AU9999",   //黄金9999",
	"AUTD",     //黄金T+D",
	"AGTD",     //白银T+D",
	"TMAP9995", //铂金9995",

	//国际行情
	"GLNC",     //美黄金",
	"PLNC",     //美铂金",
	"PANC",     //美钯金",
	"SLNC",     //美白银",
	"Unknow-2", //美铑金 ---->不用
	"Unknow-3", //未知 ---->不用
	"XAU",      //伦敦金",
	"XAG",      //伦敦银",
	"XAP",      //伦敦铂",
	"XPD",      //伦敦钯",
	"USDCNH",   //美元",
	"Unknow-4", //铑金 ---->不用
}

func TestExx_Signed(t *testing.T) {
	handle, err := Connect(
		WidgetDataWssAddress, //DataWssAddress,                     //WidgetDataWssAddress,         //wss地址
		true,
		LegalSymbolList,
		func(data BatchTicker) {

			//fmt.Printf("data:%s\n", data)

			/*
				for _, item := range data.Data {

					fmt.Printf("symbol:%s\n", item.Symbol)
					resp1, _ := json.Marshal(data)
					fmt.Printf("respose:%s\n", string(resp1))
				}

			*/
		},
		func(err error, context string) {
			fmt.Printf("%#v", "error -> "+err.Error())
			fmt.Printf("%#v", "context -> "+context)
		},
	)
	if err != nil {
		panic("Error while initializing the trading view socket -> " + err.Error())
	}

	//STOCK
	//tradingviewsocket.AddSymbols([]interface{}{"AAPL"})
	//tradingviewsocket.AddSymbols([]interface{}{"USDCNY"})
	//tradingviewsocket.AddSymbols([]interface{}{"DOGEUSDT", "BTCUSDT"})

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGTSTP, syscall.SIGKILL)
	<-quit
	handle.Close()
}
