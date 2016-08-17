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

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade err:", err)
		return
	}
	defer conn.Close()

	u := url.URL{Scheme: "ws", Host: r.Host, Path: r.URL.Path}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Print("read err:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			log.Print("readMessage err:", err)
			break
		}
		log.Print("receive :", string(msg), mt)

	}
}

func main() {
	flag.Parse()

	domains = strings.Split(domain, ",")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
