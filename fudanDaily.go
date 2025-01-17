/**
 * @Author Oliver
 * @Date 1/25/22
 **/

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/o9ltop/common/baiduAPI"
	"github.com/o9ltop/common/mail"
	"github.com/o9ltop/common/util"
	"github.com/tidwall/gjson"
)

var (
	client        *http.Client
	fudanDailyUrl = "https://zlapp.fudan.edu.cn/site/ncov/fudanDaily"
	loginUrl      = "https://uis.fudan.edu.cn/authserver/login?service=https%3A%2F%2Fzlapp.fudan.edu.cn%2Fa_fudanzlapp%2Fapi%2Fsso%2Findex%3Fredirect%3Dhttps%253A%252F%252Fzlapp.fudan.edu.cn%252Fsite%252Fncov%252FfudanDaily%26from%3Dwap"
	getInfoUrl    = "https://zlapp.fudan.edu.cn/ncov/wap/fudan/get-info"
	saveLogUrl    = "https://zlapp.fudan.edu.cn/wap/log/save-log"
	saveUrl       = "https://zlapp.fudan.edu.cn/ncov/wap/fudan/save"
	userAgent     = "Mozilla/5.0 (iPhone; CPU iPhone OS 14_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/7.0.18(0x17001229) NetType/WIFI Language/zh_CN miniProgram"
	origin        = "https://zlapp.fudan.edu.cn"
	captchaUrl    = "https://zlapp.fudan.edu.cn/backend/default/code"
	ContentType   = "application/x-www-form-urlencoded"
	Referer       = fudanDailyUrl
	gCurCookies   []*http.Cookie
	gCurCookieJar *cookiejar.Jar
	times         = 4 //验证码识别次数
	userFilePath  = "./config/"
	userFileName  = "user.json"
	userFile      = userFilePath + userFileName
	success       = `{"e":0,"m":"操作成功","d":{}}`
)

type userInfo struct {
	Username string
	Password string
	Email    string
}

func createUserfile(userFile string) (res []userInfo) {
	for {
		user := userInfo{}
		fmt.Println("请输入账号")
		fmt.Scanln(&user.Username)
		fmt.Println("请输入密码")
		fmt.Scanln(&user.Password)
		fmt.Println("请输入邮箱")
		fmt.Scanln(&user.Email)
		fmt.Println("是否继续添加y/n")
		isContinue := ""
		for {
			fmt.Scanln(&isContinue)
			if isContinue != "y" && isContinue != "n" {
				fmt.Println("错误输入，是否继续添加y/n")
			} else {
				break
			}

		}
		res = append(res, user)
		if isContinue == "n" {
			break
		}
	}
	mp := map[string]interface{}{}
	for _, u := range res {
		mp[u.Username] = []string{u.Password, u.Email}
	}
	os.MkdirAll(userFilePath, 0777)
	util.WriteToJsonFile(userFile, mp)
	return res
}

func getUsers() (res []userInfo) {
	data, _ := ioutil.ReadFile(userFile)
	if data == nil {
		fmt.Println("未发现用户数据")
		return createUserfile(userFile)
	}
	mp := util.ReadFromJsonFile(userFile)
	for k, v := range mp {
		user := userInfo{k, (v.([]interface{})[0]).(string), (v.([]interface{})[1]).(string)}
		res = append(res, user)
	}
	return res
}

/*设置请求头*/
func setHeader(r *http.Request) {
	r.Header.Add("User-Agent", userAgent)
	r.Header.Add("Origin", origin)
	r.Header.Add("Referer", Referer)
	r.Header.Add("Content-Type", ContentType)
}

/*设置验证码请求头*/
func setCaptchaHeader(r *http.Request) {
	setHeader(r)
	r.Header.Add("accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	r.Header.Add("accept-encoding", "gzip")
	r.Header.Add("accept-language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	r.Header.Add("dnt", "1")
	r.Header.Add("sec-ch-ua", `"Chromium";v="92", " Not A;Brand";v="99", "Google Chrome";v="92"`)
	r.Header.Add("sec-ch-ua-mobile", "?0")
	r.Header.Add("sec-fetch-dest", "image")
	r.Header.Add("sec-fetch-mode", "no-cors")
	r.Header.Add("sec-fetch-site", "same-origin")
}

/*初始化client*/
func initClient() {
	gCurCookieJar, _ = cookiejar.New(nil)
	client = &http.Client{
		CheckRedirect: nil,
		Jar:           gCurCookieJar,
	}
}

/*登陆*/
func login(info userInfo) error {
	req, err := http.NewRequest("GET", loginUrl, nil)
	if err != nil {
		return err
	}
	setHeader(req)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	//找到表单中的所有参数按照默认填入
	uv := ""
	h, err := htmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	a := htmlquery.Find(h, "//input")
	for i := range a {
		name := htmlquery.SelectAttr(a[i], "name")
		value := htmlquery.SelectAttr(a[i], "value")
		if name != "" && name != "captchaResponse" {
			if name == "username" {
				uv += "&" + name + "=" + info.Username
			} else if name == "password" {
				uv += "&" + name + "=" + info.Password
			} else {
				uv += "&" + name + "=" + value
			}
		}
	}
	uv = uv[1:]
	req, err = http.NewRequest("POST", loginUrl, bytes.NewReader([]byte(uv)))
	if err != nil {
		return err
	}
	setHeader(req)
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	gCurCookies = gCurCookieJar.Cookies(req.URL)
	return nil
}

/*获取历史信息*/
func getHistoryInfo() (string, error) {
	req, err := http.NewRequest("GET", getInfoUrl, nil)
	if err != nil {
		return "", err
	}
	setHeader(req)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return util.ReadJson(res), nil
}

/*说去验证码图片*/
func getcaptchaData() (res []byte, err error) {
	req, err := http.NewRequest("GET", captchaUrl, nil)
	if err != nil {
		return nil, err
	}
	setCaptchaHeader(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	img, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	res = []byte(base64.StdEncoding.EncodeToString(img))
	return res, nil
}

/*获取今日的时间格式YYYYMMDD*/
func getTodayDate() string {
	d := time.Now().String()
	return d[0:4] + d[5:7] + d[8:10]
}

/*获取打卡表单数据*/
func getPayload(history string) map[string]string {
	jsonMap := gjson.Get(history, "d")
	res := map[string]string{}
	for k, v := range jsonMap.Get("info").Map() {
		res[k] = v.String()
	}
	res["realname"] = jsonMap.Get("uinfo.realname").String()
	res["number"] = jsonMap.Get("uinfo.role.number").String()
	res["ismoved"] = "0"
	res["sfhbtl"] = "0"
	res["sfjcgrq"] = "0"
	res["sfzx"] = "0"
	if res["jrdqjcqk"] != "" {
		delete(res, "jrdqjcqk")
	}
	if res["jrdqtlqk"] != "" {
		delete(res, "jrdqtlqk")
	}
	if res["area"] == "" {
		res["area"] = jsonMap.Get("oldInfo.area").String()
		res["city"] = jsonMap.Get("oldInfo.city").String()
		res["province"] = jsonMap.Get("oldInfo.province").String()
	}
	return res
}

/*签到*/
func signIn(data map[string]string) string {
	uv := url.Values{}
	for k, v := range data {
		uv.Add(k, v)
	}
	req, _ := http.NewRequest("POST", saveUrl, bytes.NewReader([]byte(uv.Encode())))
	setHeader(req)
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}

func main() {
	users := getUsers()
	m := mail.NewMail()
	b := baiduAPI.NewBaiduAPI()
	for _, user := range users {
		initClient()
		err := login(user)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		history, err := getHistoryInfo()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		if history == "" {
			fmt.Println(user.Username, "未获取到历史数据，请检查账号密码是否正确")
			continue
		}
		data := getPayload(history)
		if data["date"] == getTodayDate() {
			fmt.Println(`今日已打卡` + "\n姓名:    " + data["realname"] + "\n地点:    " + data["address"])
			msg := "</br>姓名:    " + data["realname"] + "</br>地点:    " + data["address"]
			m.MailTo(user.Email, "今日已打卡:"+data["area"], msg)
			continue
		}
		var (
			flag    bool
			message string
		)
		for i := 0; i < times; i++ {
			img, err := getcaptchaData()
			if err != nil {
				fmt.Println(err.Error())
				continue
				return
			}
			ans := b.Recognize(img)
			data["sfz"] = "1"
			data["code"] = ans
			message = signIn(data)
			if string(message) == success {
				msg := `验证码识别` + strconv.Itoa(i+1) + "次" + "</br>姓名:    " + data["realname"] + "</br>地点:    " + data["address"]
				m.MailTo(user.Email, "打卡成功:"+data["area"], msg)
				fmt.Println("打卡成功", `验证码识别`+strconv.Itoa(i+1)+"次"+"\n姓名:    "+data["realname"]+"\n地点:    "+data["address"])
				flag = true
				break
			} else {
				fmt.Println(message)
			}
		}
		if !flag {
			msg := "打卡失败请手动打卡"
			m.MailTo(user.Email, msg, msg)
			fmt.Println(msg)
		}
		os.MkdirAll(userFilePath, 0777)
		ioutil.WriteFile(userFilePath+user.Username+".json", []byte(history), 0777)
	}
}
