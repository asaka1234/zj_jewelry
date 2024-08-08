package zj_jewelry

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestExx_Signed(t *testing.T) {
	_, err := Connect(
		WidgetDataWssAddress, //DataWssAddress,                     //WidgetDataWssAddress,         //wss地址
		func(data BatchTicker) {

			fmt.Printf("data:%s\n", data)

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
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)
	<-quit
}
