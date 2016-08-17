package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

var (
	port    string
	domain  string
	domains []string
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

func main() {
	flag.Parse()

	domains = strings.Split(domain, ",")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		client := &http.Client{}
		r.RequestURI = ""
		res, err := client.Do(r)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("remote service error: %v", err)))
			return
		}
		defer res.Body.Close()
		for k, _ := range res.Header {
			w.Header().Set(k, res.Header.Get(k))
		}
		body, _ := ioutil.ReadAll(res.Body)
		pkg := fmt.Sprintf("[%s]", r.URL.String())

		for k, _ := range r.Form {
			value := r.Form.Get(k)
			pkg += fmt.Sprintf("\t{%s=>%s}", k, value)
		}

		strbody := string(body)
		if len(strbody) < 1024 {
			pkg += "[" + strbody + "]"
		}
		monitor([]byte(pkg), r)
		w.Write(body)
	})

	log.Print("webproxy start ...")
	http.ListenAndServe(":"+port, nil)
}
