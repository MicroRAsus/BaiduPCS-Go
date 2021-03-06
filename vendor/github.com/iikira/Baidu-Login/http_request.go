package baidulogin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/iikira/Baidu-Login/bdcrypto"
	"github.com/iikira/BaiduPCS-Go/pcsutil"
	"github.com/iikira/BaiduPCS-Go/requester"
	"net/http"
	"net/http/cookiejar"
	"regexp"
)

type BaiduClient struct {
	*requester.HTTPClient

	serverTime string
	traceid    string
}

type LoginJSON struct {
	ErrInfo struct {
		No  string `json:"no"`
		Msg string `json:"msg"`
	} `json:"errInfo"`
	Data struct {
		CodeString   string `json:"codeString"`
		GotoURL      string `json:"gotoUrl"`
		Token        string `json:"token"`
		U            string `json:"u"`
		AuthSID      string `json:"authsid"`
		Phone        string `json:"phone"`
		Email        string `json:"email"`
		BDUSS        string `json:"bduss"`
		PToken       string `json:"ptoken"`
		SToken       string `json:"stoken"`
		CookieString string `json:"cookieString"`
	} `json:"data"`
}

func NewBaiduClinet() *BaiduClient {
	bc := &BaiduClient{
		HTTPClient: requester.NewHTTPClient(),
	}

	bc.getServerTime() // 访问一次百度页面，以初始化百度的 Cookie
	bc.getTraceID()
	return bc
}

// baiduLogin 发送 百度登录请求
func (bc *BaiduClient) BaiduLogin(username, password, verifycode, vcodestr string) (lj *LoginJSON) {
	isPhone := "0"
	if pcsutil.ChinaPhoneRE.MatchString(username) {
		isPhone = "1"
	}

	enpass, err := bdcrypto.RSAEncrypt(bc.getBaiduRSAPublicKeyModulus(), []byte(password+bc.serverTime))
	if err != nil {
		lj.ErrInfo.No = "-1"
		lj.ErrInfo.Msg = "RSA加密失败, " + err.Error()
		return lj
	}

	post := map[string]string{
		"username":   username,
		"password":   fmt.Sprintf("%x", enpass),
		"verifycode": verifycode,
		"vcodestr":   vcodestr,
		"isphone":    isPhone,
		"action":     "login",
		"uid":        "1516806244773_357",
		"skin":       "default_v2",
		"connect":    "0",
		"dv":         "tk0.408376350146535171516806245342@oov0QqrkqfOuwaCIxUELn3oYlSOI8f51tbnGy-nk3crkqfOuwaCIxUou2iobENoYBf51tb4Gy-nk3cuv0ounk5vrkBynGyvn1QzruvN6z3drLJi6LsdFIe3rkt~4Lyz5ktfn1Qlrk5v5D5fOuwaCIxUobJWOI3~rkt~4Lyi5kBfni0vrk8~n15fOuwaCIxUobJWOI3~rkt~4Lyz5DQfn1oxrk0v5k5eruvN6z3drLneFYeVEmy-nk3c-qq6Cqw3h7CChwvi5-y-rkFizvmEufyr1By4k5bn15e5k0~n18inD0b5D8vn1Tyn1t~nD5~5T__ivmCpA~op5gr-wbFLhyFLnirYsSCIAerYnNOGcfEIlQ6I6VOYJQIvh515f51tf5DBv5-yln15f5DFy5myl5kqf5DFy5myvnktxrkT-5T__Hv0nq5myv5myv4my-nWy-4my~n-yz5myz4Gyx4myv5k0f5Dqirk0ynWyv5iTf5DB~rk0z5Gyv4kTf5DQxrkty5Gy-5iQf51B-rkt~4B__",
		"getpassUrl": "/passport/getpass?clientfrom=&adapter=0&ssid=&from=&authsite=&bd_page_type=&uid=1501513545973_702&pu=&tpl=wimn&u=https://m.baidu.com/usrprofile%3Fuid%3D1501513545973_702%23logined&type=&bdcm=060d5ffd462309f7e5529822720e0cf3d7cad665&tn=&regist_mode=&login_share_strategy=&subpro=wimn&skin=default_v2&client=&connect=0&smsLoginLink=1&loginLink=&bindToSmsLogin=&overseas=&is_voice_sms=&subpro=wimn&hideSLogin=&forcesetpwd=&regdomestic=",
		"mobilenum":  "undefined",
		"servertime": bc.serverTime,
		// "gid":          "7B3E207-25FD-4DA7-B482-A4039C935C86",
		"logLoginType": "wap_loginTouch",
		"FP_UID":       "0b58c206c9faa8349576163341ef1321",
		"traceid":      bc.traceid,
	}

	header := map[string]string{
		"Content-Type":     "application/x-www-form-urlencoded",
		"User-Agent":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36",
		"Accept":           "application/json",
		"Referer":          "https://wappass.baidu.com/",
		"X-Requested-With": "XMLHttpRequest",
		"Connection":       "keep-alive",
	}

	body, err := bc.Fetch("POST", "https://wappass.baidu.com/wp/api/login", post, header)
	if err != nil {
		lj.ErrInfo.No = "-1"
		lj.ErrInfo.Msg = "网络请求失败, " + err.Error()
		return lj
	}

	// 如果 json 解析出错
	if err = json.Unmarshal(body, &lj); err != nil {
		lj.ErrInfo.No = "-1"
		lj.ErrInfo.Msg = "发送登录请求错误: " + err.Error()
		return lj
	}

	switch lj.ErrInfo.No {
	case "0":
		lj.parseCookies("https://wappass.baidu.com", bc.Jar.(*cookiejar.Jar)) // 解析登录数据
	case "400023", "400101": // 需要验证手机或邮箱
		lj.parsePhoneAndEmail(bc)
	}

	return lj
}

func (bc *BaiduClient) SendCodeToUser(verifyType, token string) (msg string) {
	url := fmt.Sprintf("https://wappass.baidu.com/passport/authwidget?action=send&tpl=&type=%s&token=%s&from=&skin=&clientfrom=&adapter=2&updatessn=&bindToSmsLogin=&upsms=&finance=", verifyType, token)
	body, err := bc.Fetch("GET", url, nil, nil)
	if err != nil {
		return err.Error()
	}

	rawMsg := regexp.MustCompile(`<p class="mod-tipinfo-subtitle">\s+(.*?)\s+</p>`).FindSubmatch(body)
	if len(rawMsg) >= 1 {
		return string(rawMsg[1])
	}

	return "未知消息"
}

func (bc *BaiduClient) VerifyCode(verifyType, token, vcode, u string) (lj *LoginJSON) {
	header := map[string]string{
		"Connection":                "keep-alive",
		"Host":                      "wappass.baidu.com",
		"Pragma":                    "no-cache",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36",
	}

	url := fmt.Sprintf("https://wappass.baidu.com/passport/authwidget?v=1501743656994&vcode=%s&token=%s&u=%s&action=check&type=%s&tpl=&skin=&clientfrom=&adapter=2&updatessn=&bindToSmsLogin=&isnew=&card_no=&finance=&callback=%s", vcode, token, u, verifyType, "jsonp1")
	body, err := bc.Fetch("GET", url, nil, header)
	if err != nil {
		lj.ErrInfo.No = "-2"
		lj.ErrInfo.Msg = "网络请求错误: " + err.Error()
		return
	}

	// 去除 body 的 callback 嵌套 "jsonp1(...)"
	body = bytes.TrimLeft(body, "jsonp1(")
	body = bytes.TrimRight(body, ")")

	// 如果 json 解析出错, 直接输出错误信息
	if err := json.Unmarshal(body, &lj); err != nil {
		lj.ErrInfo.No = "-2"
		lj.ErrInfo.Msg = "提交手机/邮箱验证码错误: " + err.Error()
		return
	}

	// 最后一步要访问的 URL
	u = fmt.Sprintf("%s&authsid=%s&fromtype=%s&bindToSmsLogin=", u, lj.Data.AuthSID, verifyType) // url

	_, err = bc.Fetch("GET", u, nil, nil)
	if err != nil {
		lj.ErrInfo.No = "-2"
		lj.ErrInfo.Msg = "提交手机/邮箱验证码错误: " + err.Error()
		return
	}

	lj.parseCookies(u, bc.Jar.(*cookiejar.Jar))
	return lj
}

func (bc *BaiduClient) getTraceID() {
	req, err := http.NewRequest("GET", "https://wappass.baidu.com/", nil)
	if err != nil {
		fmt.Println(err)
		bc.traceid = err.Error()
		return
	}

	resp, err := bc.Do(req)
	if err != nil {
		fmt.Println(err)
		bc.traceid = err.Error()
		return
	}

	resp.Body.Close()

	bc.traceid = resp.Header.Get("Trace-Id")
}

// 获取百度服务器时间, 形如 "e362bacbae"
func (bc *BaiduClient) getServerTime() {
	body, _ := bc.Fetch("GET", "https://wappass.baidu.com/wp/api/security/antireplaytoken", nil, nil)
	rawServerTime := regexp.MustCompile(`,"time":"(.*?)"`).FindSubmatch(body)
	if len(rawServerTime) >= 1 {
		bc.serverTime = string(rawServerTime[1])
		return
	}
	bc.serverTime = "e362bacbae"
}

// 获取百度 RSA 字串
func (bc *BaiduClient) getBaiduRSAPublicKeyModulus() (RSAPublicKeyModulus string) {
	body, _ := bc.Fetch("GET", "https://wappass.baidu.com/static/touch/js/login_d9bffc9.js", nil, nil)
	rawRSA := regexp.MustCompile(`,rsa:"(.*?)",error:`).FindSubmatch(body)
	if len(rawRSA) >= 1 {
		return string(rawRSA[1])
	}
	return "B3C61EBBA4659C4CE3639287EE871F1F48F7930EA977991C7AFE3CC442FEA49643212E7D570C853F368065CC57A2014666DA8AE7D493FD47D171C0D894EEE3ED7F99F6798B7FFD7B5873227038AD23E3197631A8CB642213B9F27D4901AB0D92BFA27542AE890855396ED92775255C977F5C302F1E7ED4B1E369C12CB6B1822F"
}
