package zj_jewelry

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestExx_Signed(t *testing.T) {
	_, err := Connect(
		WidgetDataWssAddress, //DataWssAddress,                     //WidgetDataWssAddress,         //wss地址
		func(data []Ticker) {

			for _, item := range data {

				fmt.Printf("symbol:%s\n", item.Symbol)
				resp1, _ := json.Marshal(data)
				fmt.Printf("respose:%s\n", string(resp1))

				/*
					if data.Low != nil {
						fmt.Printf("low=%s\n", *data.Low)
					}
					if data.High != nil {
						fmt.Printf("high=%s\n", *data.High)
					}
					//如果没有数据,证明没有任何change
					if data.Buy != nil {
						fmt.Printf("buy=%s\n", *data.Buy)
					}
					if data.Sell != nil {
						fmt.Printf("sell=%s\n", *data.Sell)
					}
					//fmt.Printf("%#v\n", *data.Price)
					if data.Name != nil {
						fmt.Printf("name=%s\n", *data.Name)
					}

				*/
			}
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
