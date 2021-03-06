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
	"sync"
	"time"
	"unicode/utf8"

	"github.com/clsung/plurgo/plurkgo"
	"github.com/garyburd/go-oauth/oauth"
)

var (
	c    string
	d    bool
	h    bool
	l    int
	opt  map[string]string
	errc int
)

const gameID = 14290126 // 人狼

func init() {
	flag.StringVar(&c, "c", "config.json", "載入設定檔")
	flag.BoolVar(&d, "d", false, "刪除所有噗")
	flag.BoolVar(&h, "h", false, "說明")
	flag.IntVar(&l, "l", -1, "紀錄")
	flag.Usage = usage
}

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
		// 迴圈開始
		for true {
			// 取得最近的噗
			opt = map[string]string{}
			opt["offset"] = time.Now().Format("2006-1-2T15:04:05") // 現在時間
			opt["limit"] = "10"                                    // 取 10 個
			opt["minimal_user"] = "true"
			ans, _ = callAPI(tok, "/APP/Timeline/getPlurks", opt)
			plurks := plurksObj{} // 抓回來的噗
			json.Unmarshal(ans, &plurks)
			isOpen := false // 是否開村
			isDone := false // 是否結束
			doOpen := false // 要不要開
			// 跑所有噗
			for _, plurk := range plurks.Plurks {
				isOpen = strings.Contains(plurk.ContentRaw, "開村") // 有開村字串
				dtOpen, _ := time.Parse(time.RFC1123, plurk.Posted)
				dfOpen := time.Now().UnixNano() - dtOpen.UnixNano() // 最後一次開村的時間差
				if isOpen {
					fmt.Printf("%s...\n%s\n",
						time.Now().Format("2006-01-02 15:04:05.000"),
						dtOpen.Local().Format("2006-01-02 15:04:05"))
					// 取得回應
					opt = map[string]string{}
					opt["plurk_id"] = strconv.Itoa(plurk.PlurkID)
					opt["minimal_user"] = "true"
					ans, _ = callAPI(tok, "/APP/Responses/get", opt)
					responses := plurksObj{}
					json.Unmarshal(ans, &responses)
					for i, response := range responses.Responses { // 每個回應
						if !isDone {
							isDone, _ = regexp.MatchString("(陣營|妖狐)存活", response.ContentRaw)
						}
						t, _ := time.Parse(time.RFC1123, response.Posted)
						r := strings.NewReplacer("\n", ", ", "**", "", "__", "")
						re := regexp.MustCompile("\\*(.+)\\*")
						response.ContentRaw = r.Replace(response.ContentRaw)
						response.ContentRaw = re.ReplaceAllString(response.ContentRaw, "${1}")
						response.ContentRaw = strings.Trim(response.ContentRaw, " ")
						var s string
						if utf8.RuneCountInString(response.ContentRaw) > 30 {
							s = fmt.Sprintf("%s...", string([]rune(response.ContentRaw)[:30]))
						} else {
							s = response.ContentRaw
						}
						if i >= responses.ResponsesSeen {
							fmt.Printf("%s, { %s }\n", t.Local().Format("2006-01-02 15:04:05"), s)
						}
						if isDone && plurk.NoComments == 0 {
							fmt.Println("結束...")
							// 記錄開始結束時間
							if l > -1 {
								dtResp, _ := time.Parse(time.RFC1123, response.Posted)
								opt = map[string]string{}
								opt["plurk_id"] = strconv.Itoa(l)
								opt["qualifier"] = ":"
								opt["content"] = fmt.Sprintf("PlurkID: %d\n開村: %s\n結束: %s\n進行 %d 秒",
									plurk.PlurkID,
									dtOpen.Local().Format("2006-01-02 15:04:05"),
									dtResp.Local().Format("2006-01-02 15:04:05"),
									(dtResp.UnixNano()-dtOpen.UnixNano())/1000000000)
								callAPI(tok, "/APP/Responses/responseAdd", opt)
							}
							// 關閉回應
							opt = map[string]string{}
							opt["plurk_id"] = strconv.Itoa(plurk.PlurkID)
							opt["no_comments"] = "1"
							callAPI(tok, "/APP/Timeline/toggleComments", opt)
						}
					}
					if !isDone && dfOpen >= 3600000000000 {
						isOpen = false
						opt = map[string]string{}
						opt["qualifier"] = ":"
						opt["lang"] = "ja"
						opt["content"] = fmt.Sprintf("廢村\n%s", time.Now().Format("2006-01-02 15:04:05.000"))
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
			// 刪除所有噗
			wg := &sync.WaitGroup{}
			for doOpen && d {
				opt = map[string]string{}
				opt["offset"] = time.Now().Format("2006-1-2T15:04:05")
				opt["limit"] = "50"
				opt["minimal_user"] = "true"
				opt["minimal_data"] = "true"
				ans, _ = callAPI(tok, "/APP/Timeline/getPlurks", opt)
				plurks := plurksObj{}
				json.Unmarshal(ans, &plurks)
				// 只剩下記錄噗
				if len(plurks.Plurks) == 1 && plurks.Plurks[0].PlurkID == l {
					break
				} else if len(plurks.Plurks) > 0 {
					for _, plurk := range plurks.Plurks {
						// 不刪除記錄噗
						if plurk.PlurkID != l {
							fmt.Printf("刪除: %d\n", plurk.PlurkID)
							opt = map[string]string{}
							opt["plurk_id"] = strconv.Itoa(plurk.PlurkID)
							wg.Add(1)
							go func() {
								defer wg.Done()
								callAPI(tok, "/APP/Timeline/plurkDelete", opt)
							}()
						}
					}
				} else {
					break
				}
				wg.Wait()
			}
			if doOpen && d {
				d = false
			}
			if doOpen {
				// 開村然後開始
				fmt.Println("開村...")
				opt = map[string]string{}
				opt["limited_to"] = fmt.Sprintf("[%d]", gameID)
				opt["qualifier"] = ":"
				opt["lang"] = "ja"
				opt["content"] = fmt.Sprintf("%s\n開村", time.Now().Format("2006-01-02 15:04:05.000"))
				opt["porn"] = "1"
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
					for i := 0; i < 14; i++ {
						opt["content"] = fmt.Sprintf("高橋李依進村\n%d, %s", i+1, time.Now().Format("2006-01-02 15:04:05.000"))
						_, e := callAPI(tok, "/APP/Responses/responseAdd", opt)
						if e != nil {
							i--
						}
						if errc > 100 {
							break
						}
						// 隨機秒數召喚
						tMin := 2000
						tMax := 4000
						t := time.Duration(rand.Intn(tMax-tMin) + tMin)
						fmt.Printf("%d, %d\n", i+1, t)
						time.Sleep(t * time.Millisecond)
					}
					// 開始
					fmt.Println("開始...")
					opt["qualifier"] = ":"
					opt["content"] = "開始"
					ans, e := callAPI(tok, "/APP/Responses/responseAdd", opt)
					if e != nil {
						fmt.Printf("%+v\n", e)
					} else {
						printJSONIndent(ans, "", "  ")
					}
				}
			} else {
				fmt.Print("等待...\n\n")
			}
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
	fmt.Printf("\n%s\n\n", jsi.Bytes())
}

func printObjIndent(data interface{}) {
	ans, _ := json.Marshal(data)
	printJSONIndent(ans, "", "  ")
}
