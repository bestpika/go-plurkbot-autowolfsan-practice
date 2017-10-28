package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/clsung/plurgo/plurkgo"
	"github.com/garyburd/go-oauth/oauth"
)

var (
	c    string
	h    bool
	opt  map[string]string
	errc int
)

func init() {
	flag.BoolVar(&h, "h", false, "說明")
	flag.StringVar(&c, "c", "config.json", "載入設定檔")
	flag.Usage = usage
}

/* 流程
1. 取最近 10 個噗
2. 依序取得回應
	2.1 如果沒有「開村」字串，開村
	2.2 有「開村」字串但已經結算，開村
	2.3 有「開村」字串沒有結算但超過 1 小時，開村
*/
func main() {
	flag.Parse()
	if h {
		flag.Usage()
	} else if c != "" {
		// 登入
		tok := plurkAuth(&c)
		// 個人資料
		opt = map[string]string{}
		opt["include_plurks"] = "false"
		ans, _ := callAPI(tok, "/APP/Profile/getOwnProfile", opt)
		plurker := plurkerObj{} // 使用者
		json.Unmarshal(ans, &plurker)
		printObjIndent(plurker)
		for true {
			// 取得最近的噗
			opt = map[string]string{}
			opt["offset"] = time.Now().Format("2006-1-2T15:04:05") // 比現在早的
			opt["limit"] = "10"                                    // 取 10 個
			opt["minimal_user"] = "true"
			ans, _ = callAPI(tok, "/APP/Timeline/getPlurks", opt)
			plurks := plurksObj{} // 抓回來的噗
			json.Unmarshal(ans, &plurks)
			// printObjIndent(plurks)
			isOpen := false // 是否開村
			isDone := false // 是否結束
			doOpen := false // 要不要開
			// 跑所有噗
			for _, plurk := range plurks.Plurks {
				isOpen = strings.Contains(plurk.ContentRaw, "開村") // 有開村字串
				dtOpen, _ := time.Parse(time.RFC1123, plurk.Posted)
				dfOpen := time.Now().UnixNano() - dtOpen.UnixNano()
				if isOpen {
					fmt.Println(dtOpen.Format("2006-01-02 15:04:05 -0700"))
					// 取得回應
					opt = map[string]string{}
					opt["plurk_id"] = strconv.Itoa(plurk.PlurkID)
					opt["minimal_user"] = "true"
					ans, _ = callAPI(tok, "/APP/Responses/get", opt)
					responses := plurksObj{}
					json.Unmarshal(ans, &responses)
					// printObjIndent(responses)
					for i, response := range responses.Responses { // 每個回應
						if !isDone {
							isDone, _ = regexp.MatchString("(陣營|妖狐)存活", response.ContentRaw)
						}
						t, _ := time.Parse(time.RFC1123, response.Posted)
						response.ContentRaw = strings.Trim(response.ContentRaw, " ")
						response.ContentRaw = strings.Replace(response.ContentRaw, "\n", ", ", -1)
						response.ContentRaw = strings.Trim(response.ContentRaw, " ")
						var s string
						if utf8.RuneCountInString(response.ContentRaw) > 30 {
							s = fmt.Sprintf("%s...", string([]rune(response.ContentRaw)[:30]))
						} else {
							s = response.ContentRaw
						}
						if i >= responses.ResponseSeen {
							fmt.Printf("%s, { %s }\n", t.Format("2006-01-02 15:04:05 -0700"), s)
						}
					}
					if !isDone && dfOpen >= 3600000000000 {
						isOpen = false
						opt = map[string]string{}
						opt["qualifier"] = ":"
						opt["content"] = fmt.Sprintf("廢村\n%s", time.Now().Format("2006/01/02 15:04:05.000"))
						opt["lang"] = "ja"
						ans, _ = callAPI(tok, "/APP/Timeline/plurkAdd", opt)
						plurk := plurkObj{}
						json.Unmarshal(ans, &plurk)
						opt = map[string]string{}
						opt["plurk_id"] = strconv.Itoa(plurk.PlurkID)
						time.Sleep(15 * time.Second)
						callAPI(tok, "/APP/Timeline/plurkDelete", opt)
					}
					break // 有開村就跳出去
				}
			}
			if !isOpen || isOpen && isDone {
				doOpen = true
			}
			if doOpen {
				// 開村然後開始
				fmt.Println("開村！")
				opt = map[string]string{}
				opt["qualifier"] = ":"
				opt["content"] = fmt.Sprintf("%s\n開村", time.Now().Format("2006/01/02 15:04:05.000"))
				opt["lang"] = "ja"
				ans, e := callAPI(tok, "/APP/Timeline/plurkAdd", opt)
				plurk := plurkObj{}
				if e != nil {
					fmt.Printf("%+v\n", e)
				} else {
					json.Unmarshal(ans, &plurk)
					// 複製人
					opt = map[string]string{}
					opt["plurk_id"] = strconv.Itoa(plurk.PlurkID)
					opt["qualifier"] = "will"
					rand.Seed(time.Now().UnixNano())
					for i := 0; i < 29; i++ {
						opt["content"] = fmt.Sprintf("高橋李依進村\n[%d] %s", i+1, time.Now().Format("2006/01/02 15:04:05.000"))
						_, e := callAPI(tok, "/APP/Responses/responseAdd", opt)
						if e != nil {
							i--
						}
						if errc > 100 {
							break
						}
						// 隨機秒數召喚
						tMin := 2000
						tMax := 5000
						t := time.Duration(rand.Intn(tMax-tMin) + tMin)
						fmt.Printf("[%d] %d\n", i+1, t)
						time.Sleep(t * time.Millisecond)
					}
					// 開始
					opt["qualifier"] = ":"
					opt["content"] = "開始"
					ans, e := callAPI(tok, "/APP/Responses/responseAdd", opt)
					fmt.Println("開始～")
					if e != nil {
						fmt.Printf("%+v\n", e)
					} else {
						printJSONIndent(ans, "", "  ")
					}
				}
			} else {
				fmt.Print("等待...")
			}
			fmt.Print("\n\n")
			time.Sleep(5 * time.Second)
		}
	}
}

func usage() {
	fmt.Printf("\n%s\n", "Options:")
	flag.PrintDefaults()
	fmt.Println()
}

func plurkAuth(credPath *string) *oauth.Credentials {
	plurkOAuth, e := plurgo.ReadCredentials(*credPath)
	if e != nil {
		log.Fatalf("%+v", e)
	}
	tok, auth, e := plurgo.GetAccessToken(plurkOAuth)
	if auth {
		b, e := json.MarshalIndent(plurkOAuth, "", "  ")
		if e != nil {
			log.Fatal(e)
		}
		e = ioutil.WriteFile(c, b, 0700)
		if e != nil {
			log.Fatal(e)
		}
	}
	return tok
}

func callAPI(tok *oauth.Credentials, api string, opt map[string]string) ([]byte, error) {
	ans, e := plurgo.CallAPI(tok, api, opt)
	if e != nil {
		errc++
		log.Fatal(e)
	} else {
		errc = 0
	}
	return ans, e
}

func printJSONIndent(data []byte, prefix, indent string) {
	var jsi bytes.Buffer
	json.Indent(&jsi, []byte(data), prefix, indent)
	fmt.Printf("%s\n\n", jsi.Bytes())
}

func printObjIndent(data interface{}) {
	ans, _ := json.Marshal(data)
	printJSONIndent(ans, "", "  ")
}
