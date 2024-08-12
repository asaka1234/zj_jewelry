package zj_jewelry

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/recws-org/recws"
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

	mx sync.Mutex
	//conn      *websocket.Conn
	conn      *recws.RecConn
	isClosed  bool
	sessionID string

	pingInterval         int //发送ping的时间间隔
	closePingChan        chan bool
	closeSendMessageChan chan bool
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
		closePingChan:               make(chan bool), //关闭发ping的task
		closeSendMessageChan:        make(chan bool), //不再定期发subMsg
		pingInterval:                20,              //默认
	}

	err = socket.Init()

	return
}

// Init connects to the tradingview web socket
func (s *TradingViewWebSocket) Init() (err error) {
	s.mx = sync.Mutex{}
	s.isClosed = true
	//ctx, cancel := context.WithCancel(context.Background())
	s.conn = &recws.RecConn{
		KeepAliveTimeout: 10 * time.Second,
	}
	s.conn.Dial(s.address, getHeaders())

	/*
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

	*/

	/*
		s.conn, _, err = (&websocket.Dialer{}).Dial(s.address, getHeaders())
		if err != nil {
			s.onError(err, InitErrorContext)
			return
		}

	*/

	//链接上服务器后, server会推过来一个初始化确认信息
	err = s.checkFirstReceivedMessage()
	if err != nil {
		return
	}

	s.isClosed = false
	go s.connectionLoop()
	go s.sendPing()
	go s.sendMessage()

	return
}

// Close ...
func (s *TradingViewWebSocket) Close() (err error) {
	s.isClosed = true
	s.closeSendMessageChan <- true
	s.closePingChan <- true
	s.conn.Close()
	return nil
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
	if p["sid"] == nil || p["pingInterval"] == nil {
		err = errors.New("cannot recognize the first received message after establishing the connection")
		s.onError(err, FirstMessageWithoutSessionIdErrorContext)
		return
	}
	s.pingInterval = int(p["pingInterval"].(float64)) //毫秒 25000  (25s)

	//先发一个ping
	pingMsg := []byte("2")
	//s.mx.Lock()
	//defer s.mx.Unlock()
	s.conn.WriteMessage(websocket.TextMessage, pingMsg)
	fmt.Printf("Send:%s\n", string(pingMsg))

	return
}

// 周期性的发送ping给服务侧,以保持链接
func (s *TradingViewWebSocket) sendPing() {

	pingMsg := []byte("2")

	//25s发一个ping
	ticker := time.NewTicker(time.Duration(s.pingInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case isClose := <-s.closePingChan:
			if isClose {
				fmt.Printf("Client, stop ping goroutine\n")
				return
			}
		case <-ticker.C:
			s.mx.Lock()
			//defer s.mx.Unlock()
			err := s.conn.WriteMessage(websocket.TextMessage, pingMsg)
			if err != nil {
				s.onError(err, SendMessageErrorContext+" - "+string(pingMsg))
				//return
			} else {
				fmt.Printf("Send:%s\n", string(pingMsg))
			}
			s.mx.Unlock()
		default:
			break
		}
	}
}

func (s *TradingViewWebSocket) sendMessage() {

	subMsg := []byte("42[\"msg\",{\"msg\":\"undefined\"}]")
	//1s发一个ping
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case isClose := <-s.closeSendMessageChan:
			if isClose {
				fmt.Printf("Client, stop send message goroutine\n")
				return
			}
		case <-ticker.C:
			s.mx.Lock()
			//defer s.mx.Unlock()
			err := s.conn.WriteMessage(websocket.TextMessage, subMsg)
			if err != nil {
				s.onError(err, SendMessageErrorContext+" - "+string(subMsg))
				//return
			} else {
				fmt.Printf("Send:%s\n", string(subMsg))
			}
			s.mx.Unlock()
		default:
			break
		}
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
				s.onError(readMsgError, MessageTypeErrorContext)
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

	fmt.Printf("Receive:%s\n", string(packet))

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
				Symbol: symbol,
				Bid:    convert2String(quote.Buy),
				Ask:    convert2String(quote.Sell),
				High:   convert2String(quote.High),
				Low:    convert2String(quote.Low),
			})
		}
	}

	//一个批量消息传递
	if len(quoteList) > 0 {
		s.OnReceiveMarketDataCallback(BatchTicker{
			//RecvTime: time.Now().Format("2006-01-02 15:04:05"), //quoteMessage.PubTime,
			PubTime: quoteMessage.PubTime, //2024-08-07 18:36:43
			Data:    quoteList,
		})
	}
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

	headers.Set("Accept-Encoding", "zip, deflate")
	headers.Set("Accept-Language", "en-US,en;q=0.9,es;q=0.8")
	headers.Set("Cache-Control", "no-cache")
	//headers.Set("Host", "data.tradingview.com")
	headers.Set("Origin", "http://ycjgjj.dsdgood.com")
	headers.Set("Pragma", "no-cache")
	headers.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")

	return headers
}
