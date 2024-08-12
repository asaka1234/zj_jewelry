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
		pingInterval:                25000,           //默认
	}

	err = socket.Init()
	if err != nil {
		return nil, err
	}

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
	/*
		err = s.checkFirstReceivedMessage()
		if err != nil {
			return err
		}

	*/

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
	//var msg []byte

	/*
			if !s.conn.IsConnected() {
				s.onError(errors.New("ws not connected-1"), InitErrorContext)
				continue
			}

			_, msg, err = s.conn.ReadMessage()
			if err != nil {
				s.onError(err, ReadFirstMessageErrorContext)
				return
			}

			//2. 解析
			err = s.parseOpenMessage(msg)
			if err != nil {
				return err
			}



		//3. 先发一个ping
		pingMsg := []byte("2")
		//s.mx.Lock()
		//defer s.mx.Unlock()
		err = s.conn.WriteMessage(websocket.TextMessage, pingMsg)
		if err == nil {
			fmt.Printf("ZJ_Lib Send:%s\n", string(pingMsg))
		} else {
			s.onError(err, SendMessageErrorContext+" - "+string(pingMsg))
		}	*/

	return
}

// 获取收到的消息类型
func (s *TradingViewWebSocket) getMessageType(msg []byte) MsgType {
	//0是open事件
	index := strings.Index(string(msg), string(MsgTypeOpen))
	if index == 0 {
		//open
		return MsgTypeOpen
	}

	//3是pong
	index = strings.Index(string(msg), string(MsgTypePong))
	if index == 0 {
		//msg
		return MsgTypePong
	}

	//42是msg
	index = strings.Index(string(msg), string(MsgTypeMessage))
	if index == 0 {
		//msg
		return MsgTypeMessage
	}

	return MsgTypeUnknown
}

func (s *TradingViewWebSocket) parseOpenMessage(msg []byte) error {

	//这个是过滤掉最前边的 数字 前缀，后边就是json了. 这个json字符串就是payload
	payload := msg[1:]
	var p map[string]interface{}
	//反序列化一下
	err := json.Unmarshal(payload, &p)
	if err != nil {
		s.onError(err, DecodeFirstMessageErrorContext)
		return err
	}

	//本质上就是看一下有没有session_id
	if p["sid"] == nil || p["pingInterval"] == nil {
		err = errors.New("cannot recognize the first received message after establishing the connection")
		s.onError(err, FirstMessageWithoutSessionIdErrorContext)
		return err
	}
	s.pingInterval = int(p["pingInterval"].(float64)) //毫秒 25000  (25s)

	return nil
}

func (s *TradingViewWebSocket) parseNormalMessage(data []byte) error {

	//var symbolsArr []string
	var quoteMessage QuoteMessage
	var parseMsg = []interface{}{"msgcallback", quoteMessage}

	//把最前边的42过滤掉
	msg := string(data)[2:]

	//解析json字符串
	err := json.Unmarshal([]byte(msg), &parseMsg)
	if err != nil {
		s.onError(err, DecodeMessageErrorContext+" - "+string(msg))
		return err
	}
	if parseMsg[0] != "msgcallback" {
		s.onError(err, DecodeMessageErrorContext+" - "+"msgcallback missed")
		return err
	}

	if err := mapstructure.Decode(parseMsg[1], &quoteMessage); err != nil {
		s.onError(err, DecodeMessageErrorContext+" - "+err.Error())
		return err
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
	return nil
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
			if !s.conn.IsConnected() {
				s.onError(errors.New("ws not connected-2"), InitErrorContext)
				continue
			}

			s.mx.Lock()
			//defer s.mx.Unlock()
			err := s.conn.WriteMessage(websocket.TextMessage, pingMsg)
			if err != nil {
				s.onError(err, SendMessageErrorContext+" - "+string(pingMsg))
				//return
			} else {
				fmt.Printf("ZJ_Lib Send:%s\n", string(pingMsg))
			}
			s.mx.Unlock()
		default:
			break
		}
	}
}

func (s *TradingViewWebSocket) sendMessage() {
	subMsg := []byte("42[\"msg\",{\"msg\":\"88888\"}]")
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
			if !s.conn.IsConnected() {
				s.onError(errors.New("ws not connected-3"), InitErrorContext)
				continue
			}

			s.mx.Lock()
			//defer s.mx.Unlock()
			err := s.conn.WriteMessage(websocket.TextMessage, subMsg)
			if err != nil {
				s.onError(err, SendMessageErrorContext+" - "+string(subMsg))
				//return
			} else {
				fmt.Printf("ZJ_Lib Send:%s\n", string(subMsg))
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
		//s.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if !s.conn.IsConnected() {
			//s.onError(errors.New("ws not connected-4"), InitErrorContext)
			continue
		}
		msgType, msg, readMsgError = s.conn.ReadMessage()

		go func(msgType int, msg []byte) {
			if msgType == websocket.TextMessage {
				//s.onError(readMsgError, MessageTypeErrorContext)
				//fmt.Printf("--ex------%d, %s\n", msgType, string(msg))
				//return
				//}
				go s.parsePacket(msg)
			}

		}(msgType, msg)
	}

	if readMsgError != nil {
		//读取不到直接断开,随后重连
		go s.conn.Close()
		//s.onError(readMsgError, ReadMessageErrorContext)
		return
	}
	if writeKeepAliveMsgError != nil {
		s.onError(writeKeepAliveMsgError, SendKeepAliveMessageErrorContext)
	}
}

// 负责解析收到的数据
func (s *TradingViewWebSocket) parsePacket(packet []byte) {

	//{"sid":"NjZiOTljNmEwOTA2YQ==","upgrades":[],"pingInterval":25000,"pingTimeout":60000}
	fmt.Printf("ZJ_Lib Receive:%s\n", string(packet))

	//查询消息类型
	msgType := s.getMessageType(packet)
	if msgType == MsgTypeUnknown {
		//s.onError(errors.New("unknown message type"), string(packet))
		return
	}

	if msgType == MsgTypeOpen {
		//open类型
		err := s.parseOpenMessage(packet)
		if err != nil {
			return
		}
	} else if msgType == MsgTypeMessage {
		//真实的消息
		err := s.parseNormalMessage(packet)
		if err != nil {
			return
		}
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
