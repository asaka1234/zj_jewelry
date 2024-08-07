package zj_jewelry

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Socket ...
type TradingViewWebSocket struct {
	address string //websocket地址

	OnReceiveMarketDataCallback OnReceiveDataCallback
	OnErrorCallback             OnErrorCallback

	mx        sync.Mutex
	conn      *websocket.Conn
	isClosed  bool
	sessionID string
}

// Connect - Connects and returns the trading view socket object
func Connect(
	address string,
	onReceiveMarketDataCallback OnReceiveDataCallback,
	onErrorCallback OnErrorCallback,
) (socket SocketInterface, err error) {
	socket = &TradingViewWebSocket{
		address:                     address,
		OnReceiveMarketDataCallback: onReceiveMarketDataCallback,
		OnErrorCallback:             onErrorCallback,
	}

	err = socket.Init()

	return
}

// Init connects to the tradingview web socket
func (s *TradingViewWebSocket) Init() (err error) {
	s.mx = sync.Mutex{}
	s.isClosed = true
	s.conn, _, err = (&websocket.Dialer{}).Dial(s.address, getHeaders())
	if err != nil {
		s.onError(err, InitErrorContext)
		return
	}

	//链接上服务器后, server会推过来一个初始化确认信息
	err = s.checkFirstReceivedMessage()
	if err != nil {
		return
	}

	//创建一个session_id,这个是这次wss交互的唯一标记
	//s.generateSessionID()

	/*
		err = s.sendConnectionSetupMessages()
		if err != nil {
			s.onError(err, ConnectionSetupMessagesErrorContext)
			return
		}

	*/

	s.isClosed = false
	go s.connectionLoop()
	go s.sendPing()

	return
}

// Close ...
func (s *TradingViewWebSocket) Close() (err error) {
	s.isClosed = true
	return s.conn.Close()
}

func (s *TradingViewWebSocket) checkFirstReceivedMessage() (err error) {
	var msg []byte

	_, msg, err = s.conn.ReadMessage()
	if err != nil {
		s.onError(err, ReadFirstMessageErrorContext)
		return
	}

	index := strings.Index(string(msg), "{")
	if index == -1 {
		//暂时不处理
		return nil
	}

	//这个是过滤掉最前边的 数字 前缀，后边就是json了. 这个json字符串就是payload
	payload := msg[index:]
	var p map[string]interface{}
	//反序列化一下
	err = json.Unmarshal(payload, &p)
	if err != nil {
		s.onError(err, DecodeFirstMessageErrorContext)
		return
	}

	//本质上就是看一下有没有session_id
	if p["sid"] == nil {
		err = errors.New("cannot recognize the first received message after establishing the connection")
		s.onError(err, FirstMessageWithoutSessionIdErrorContext)
		return
	}

	return
}

// 周期性的发送ping给服务侧,以保持链接
func (s *TradingViewWebSocket) sendPing() {

	for {
		err := s.conn.WriteMessage(websocket.TextMessage, []byte("[\"msg\",{\"msg\":\"88888\"}]"))
		if err != nil {
			//s.onError(err, SendMessageErrorContext+" - "+payloadWithHeader)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func (s *TradingViewWebSocket) connectionLoop() {
	var readMsgError error
	var writeKeepAliveMsgError error

	for readMsgError == nil && writeKeepAliveMsgError == nil {
		if s.isClosed {
			break
		}

		var msgType int
		var msg []byte
		msgType, msg, readMsgError = s.conn.ReadMessage()

		go func(msgType int, msg []byte) {
			if msgType != websocket.TextMessage {
				return
			}

			go s.parsePacket(msg)

		}(msgType, msg)
	}

	if readMsgError != nil {
		s.onError(readMsgError, ReadMessageErrorContext)
	}
	if writeKeepAliveMsgError != nil {
		s.onError(writeKeepAliveMsgError, SendKeepAliveMessageErrorContext)
	}
}

// 负责解析收到的数据
func (s *TradingViewWebSocket) parsePacket(packet []byte) {
	//var symbolsArr []string
	var quoteMessage QuoteMessage
	var parseMsg = []interface{}{"msgcallback", quoteMessage}

	//把空字符串也过滤掉
	index := strings.Index(string(packet), "[")
	if index == -1 {
		//暂时不处理
		return
	}

	msg := string(packet)[index:]

	//解析json字符串
	err := json.Unmarshal([]byte(msg), &parseMsg)
	if err != nil {
		s.onError(err, DecodeMessageErrorContext+" - "+string(msg))
		return
	}
	if parseMsg[0] != "msgcallback" {
		s.onError(err, DecodeMessageErrorContext+" - "+"msgcallback missed")
		return
	}

	if err := mapstructure.Decode(parseMsg[1], &quoteMessage); err != nil {
		s.onError(err, DecodeMessageErrorContext+" - "+err.Error())
		return
	}

	//批量处理
	quoteList := make([]Ticker, 0)
	for symbol, quote := range quoteMessage.Items {
		if _, ok := LegalSymbolMap[symbol]; ok {

			//批量一下
			quoteList = append(quoteList, Ticker{
				Symbol:  symbol,
				Bid:     convert2String(quote.Buy),
				Ask:     convert2String(quote.Sell),
				High:    convert2String(quote.High),
				Low:     convert2String(quote.Low),
				PubTime: quoteMessage.PubTime, //2024-08-07 18:36:43
			})
		}
	}

	//一个批量消息传递
	s.OnReceiveMarketDataCallback(quoteList)

	/*
		//批量的传递
		for symbol, quote := range quoteMessage.Sid {
			if _, ok := LegalSymbolMap[symbol]; ok {
				s.OnReceiveMarketDataCallback(symbol, quote)
			}
		}
	*/

}

// 辅助函数
func convert2String(src *string) string {
	dst := "-"
	if src != nil {
		dst = *src
	}
	return dst
}

func (s *TradingViewWebSocket) onError(err error, context string) {
	if s.conn != nil {
		s.conn.Close()
	}
	s.OnErrorCallback(err, context)
}

func getHeaders() http.Header {
	headers := http.Header{}

	headers.Set("Accept-Encoding", "gzip, deflate, br")
	headers.Set("Accept-Language", "en-US,en;q=0.9,es;q=0.8")
	headers.Set("Cache-Control", "no-cache")
	//headers.Set("Host", "data.tradingview.com")
	headers.Set("Origin", "http://ycjgjj.dsdgood.com")
	headers.Set("Pragma", "no-cache")
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.193 Safari/537.36")

	return headers
}
