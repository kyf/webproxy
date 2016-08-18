package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

var (
	port    string
	domain  string
	domains []string

	upgrader websocket.Upgrader = websocket.Upgrader{}
)

func init() {
	flag.StringVar(&port, "port", "8888", "代理服务监听端口")
	flag.StringVar(&domain, "domain", "", "指定要监控的域名")
}

func SliceContain(list []string, host string) bool {
	for _, item := range list {
		if strings.Contains(host, item) {
			return true
		}
	}

	return false
}

func monitor(data []byte, r *http.Request) {
	if len(domain) != 0 {
		if SliceContain(domains, r.Host) {
			log.Print(string(data))
		}
		return
	}

	log.Print(string(data))
}

type WebSocketMsgType int

func (this WebSocketMsgType) String() string {
	switch this {
	case 1:
		return "TextMessage"
	case 2:
		return "BinaryMessage"
	case 8:
		return "CloseMessage"
	case 9:
		return "PingMessage"
	case 10:
		return "PongMessage"
	}

	return "unknown"
}

func readMessage(conn *websocket.Conn, msg chan<- string, mt, exit chan<- int) {
	for {
		_mt, _msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read message error:%v", err)
			exit <- 1
			return
		}
		log.Printf("[%s][%s]", WebSocketMsgType(_mt), string(_msg))
		msg <- string(_msg)
		mt <- _mt
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	connReq, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade err:", err)
		return
	}
	defer connReq.Close()

	u := url.URL{Scheme: "ws", Host: r.Host, Path: r.URL.Path}
	log.Printf("connecting to %s", u.String())

	connRes, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Print("dial:", err)
		return
	}
	defer connRes.Close()

	exitReq := make(chan int, 1)
	exitRes := make(chan int, 1)

	mtReq := make(chan int, 1)
	mtRes := make(chan int, 1)

	msgReq := make(chan string, 1)
	msgRes := make(chan string, 1)

	go readMessage(connReq, msgReq, mtReq, exitReq)
	go readMessage(connRes, msgRes, mtRes, exitRes)

	for {
		select {
		case <-exitRes:
			goto Exit
		case <-exitReq:
			goto Exit
		case mReq := <-msgReq:
			if err := connRes.WriteMessage(<-mtReq, []byte(mReq)); err != nil {
				log.Printf("write message error:%v", err)
				goto Exit
			}
		case mRes := <-msgRes:
			if err := connReq.WriteMessage(<-mtRes, []byte(mRes)); err != nil {
				log.Printf("write message error:%v", err)
				goto Exit
			}
		}
	}

Exit:
	log.Print(r.RequestURI, " closed>>>>>>>>>>")
}

func main() {
	flag.Parse()

	domains = strings.Split(domain, ",")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//log.Print(r.RequestURI)
		if strings.EqualFold("websocket", r.Header.Get("Upgrade")) {
			wsHandler(w, r)
			return
		}
		client := &http.Client{}
		r.RequestURI = ""
		res, err := client.Do(r)
		if err != nil {
			log.Printf("remote service error: %v", err)
			w.Write([]byte(fmt.Sprintf("remote service error: %v", err)))
			return
		}
		defer res.Body.Close()
		for k, _ := range res.Header {
			w.Header().Set(k, res.Header.Get(k))
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("read body error: %v", err)
			w.Write([]byte(fmt.Sprintf("read body error: %v", err)))
			return
		}
		pkg := fmt.Sprintf("[%s]", r.URL.String())

		for k, _ := range r.Form {
			value := r.Form.Get(k)
			pkg += fmt.Sprintf("\t{%s=>%s}", k, value)
		}

		strbody := string(body)
		maxSize := 2 << 10
		if len(strbody) < maxSize {
			pkg += "[" + strbody + "]"
		}
		monitor([]byte(pkg), r)
		w.Write(body)
	})

	log.Print("webproxy start ...")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
