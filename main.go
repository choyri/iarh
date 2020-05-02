package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type RespCommon struct {
	Code string `json:"code"`
}

type RespCollectorProcessingList struct {
	RespCommon
	Datas struct {
		Rows []struct {
			Wid        string `json:"wid"`
			FormWid    string `json:"formWid"`
			Subject    string `json:"subject"`
			CreateTime string `json:"createTime"`
			IsHandled  int    `json:"isHandled"`
		} `json:"rows"`
	} `json:"datas"`
}

type RespDetailCollector struct {
	RespCommon
	Datas struct {
		Collector struct {
			SchoolTaskWid string `json:"schoolTaskWid"`
		} `json:"collector"`
	} `json:"datas"`
}

type RespGetFormFields struct {
	RespCommon
	Datas struct {
		Rows []RespGetFormFieldsRow `json:"rows"`
	} `json:"datas"`
}

type RespGetFormFieldsRow struct {
	Wid           string      `json:"wid"`
	FormWid       string      `json:"formWid"`
	FieldType     int         `json:"fieldType"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	MinLength     int         `json:"minLength"`
	Sort          string      `json:"sort"`
	MaxLength     int         `json:"maxLength"`
	IsRequired    int         `json:"isRequired"`
	ImageCount    int         `json:"imageCount"`
	HasOtherItems int         `json:"hasOtherItems"`
	ColName       string      `json:"colName"`
	Value         string      `json:"value"`
	FieldItems    []fieldItem `json:"fieldItems"`
	Area1         string      `json:"area1,omitempty"`
	Area2         string      `json:"area2,omitempty"`
	Area3         string      `json:"area3,omitempty"`
}

type fieldItem struct {
	ItemWid       string  `json:"itemWid"`
	Content       string  `json:"content"`
	IsOtherItems  int     `json:"isOtherItems"`
	ContendExtend *string `json:"contendExtend"`
	IsSelected    *int    `json:"isSelected"`
}

type Form struct {
	FormWid       string                 `json:"formWid"`
	Address       string                 `json:"address"`
	CollectWid    string                 `json:"collectWid"`
	SchoolTaskWid string                 `json:"schoolTaskWid"`
	Form          []RespGetFormFieldsRow `json:"form"`
}

const (
	EnvFileName     = "env.txt"
	EnvKeyDomain    = "DOMAIN"
	EnvKeyUserAgent = "USER_AGENT"
	EnvKeyExtension = "EXTENSION"
	EnvKeyCookie    = "COOKIE"
	EnvKeyAddress   = "ADDRESS"
	EnvKeyArea      = "AREA"

	URLQueryCollectorProcessingList = "/wec-counselor-collector-apps/stu/collector/queryCollectorProcessingList"
	URLDetailCollector              = "/wec-counselor-collector-apps/stu/collector/detailCollector"
	URLGetFormFields                = "/wec-counselor-collector-apps/stu/collector/getFormFields"
	URLSubmitForm                   = "/wec-counselor-collector-apps/stu/collector/submitForm"
	URLUserInfo                     = "https://mobile.campushoy.com/v6/user/myMainPage"
	URLOAuth                        = "https://www.cpdaily.com/connect/oauth2/authorize?response_type=code&client_id=15809557517376149&scope=get_user_info&state=uag&redirect_uri=https:%2F%2Fhzu1.cpdaily.com%2Fwec-counselor-collector-apps%2Fstu%2Fmobile%2Findex.html"
)

var (
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.String() == renderURL(URLOAuth) {
				return nil
			}
			return http.ErrUseLastResponse
		},
	}

	envMap map[string]string
	form   Form
)

func init() {
	var err error
	envMap, err = godotenv.Read(EnvFileName)
	if err != nil {
		log.Fatalf("读取配置文件失败：%s", err.Error())
	}
}

func main() {
	checkCookie()
	printName()
	renderForm()
	submit()
}

func checkCookie() {
	var (
		err          error
		resp         *http.Response
		refreshTimes uint
	)

begin:

	_, resp, err = post(URLQueryCollectorProcessingList, nil)
	if err != nil {
		log.Fatalf("请求失败：%s", err.Error())
	}

	if resp.StatusCode == http.StatusOK {
		if refreshTimes > 0 {
			log.Println("Cookie 刷新成功")
		}

		return
	}

	if refreshTimes >= 3 {
		log.Fatalln("Cookie 刷新失败，重试次数达到上限，请自行检查")
	}

	if refreshTimes > 0 {
		log.Println("Cookie 刷新失败，稍后将重试")
		time.Sleep(time.Second * 2)
	}

	log.Println("Cookie 失效，尝试刷新")

	_, resp, _ = get(URLOAuth)
	if l := resp.Header.Get("Location"); l != "" {
		_, _, _ = get(l)
	}

	refreshTimes++

	goto begin
}

func printName() {
	var (
		err     error
		rawData []byte
	)

	rawData, _, err = post(URLUserInfo, nil)
	if err != nil {
		log.Fatalf("获取个人信息失败：%s", err.Error())
	}

	var resp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}

	err = json.Unmarshal(rawData, &resp)
	if err != nil {
		log.Fatalf("个人信息解析失败：%s", err.Error())
	}

	log.Printf("当前用户：%s", resp.Data.Name)
}

func renderForm() {
	var (
		err     error
		rawData []byte

		collectorProcessingList RespCollectorProcessingList
		detailCollector         RespDetailCollector
		formFields              RespGetFormFields

		finalForm []RespGetFormFieldsRow
	)

	rawData, _, err = post(URLQueryCollectorProcessingList, map[string]interface{}{
		"pageSize":   10,
		"pageNumber": 1,
	})
	if err != nil {
		log.Fatalf("QueryCollectorProcessingList 请求失败：%s", err.Error())
	}

	err = json.Unmarshal(rawData, &collectorProcessingList)
	if err != nil {
		log.Fatalf("QueryCollectorProcessingList 解析失败：%s", err.Error())
	}

	if collectorProcessingList.Code != "0" {
		log.Fatalf("QueryCollectorProcessingList 请求报错：%v", collectorProcessingList)
	}

	if len(collectorProcessingList.Datas.Rows) == 0 {
		log.Println("无待处理的表单")
		os.Exit(0)
	}

	var (
		lists              = []string{"当前表单："}
		shouldHandleAmount int
	)

	for _, v := range collectorProcessingList.Datas.Rows {
		state := "已填写"
		if v.IsHandled == 0 {
			state = "未处理"
			shouldHandleAmount++
		}

		lists = append(lists, fmt.Sprintf("%s / 发布时间：%s - %s", v.Subject, v.CreateTime, state))
	}

	log.Print(strings.Join(lists, "\n                    "))

	if shouldHandleAmount == 0 {
		log.Println("无待处理的表单")
		os.Exit(0)
	}

	form.CollectWid = collectorProcessingList.Datas.Rows[0].Wid
	form.FormWid = collectorProcessingList.Datas.Rows[0].FormWid

	rawData, _, err = post(URLDetailCollector, map[string]interface{}{
		"collectorWid": form.CollectWid,
	})
	if err != nil {
		log.Fatalf("DetailCollector 请求失败：%s", err.Error())
	}

	err = json.Unmarshal(rawData, &detailCollector)
	if err != nil {
		log.Fatalf("DetailCollector 解析失败：%s", err.Error())
	}

	if detailCollector.Code != "0" {
		log.Fatalf("DetailCollector 请求报错：%v", detailCollector)
	}

	form.SchoolTaskWid = detailCollector.Datas.Collector.SchoolTaskWid

	rawData, _, err = post(URLGetFormFields, map[string]interface{}{
		"pageSize":     10,
		"pageNumber":   1,
		"formWid":      form.FormWid,
		"collectorWid": form.CollectWid,
	})
	if err != nil {
		log.Fatalf("GetFormFields 请求失败：%s", err.Error())
	}

	err = json.Unmarshal(rawData, &formFields)
	if err != nil {
		log.Fatalf("GetFormFields 解析失败：%s", err.Error())
	}

	if formFields.Code != "0" {
		log.Fatalf("GetFormFields 请求报错：%v", formFields)
	}

	for _, v := range formFields.Datas.Rows {
		v := v

		if strings.Contains(v.Title, "所在的地区") {
			v.Value = envMap[EnvKeyArea]

			areas := strings.Split(v.Value, "/")
			if len(areas) != 3 {
				log.Fatalf("env area 格式错误")
			}

			v.Area1, v.Area2, v.Area3 = areas[0], areas[1], areas[2]

			finalForm = append(finalForm, v)
			continue
		}

		if strings.Contains(v.Title, "当前的状况") {
			for kk, vv := range v.FieldItems {
				if strings.Contains(vv.Content, "正常") {
					v.Value = v.FieldItems[kk].Content
					v.FieldItems = v.FieldItems[kk : kk+1]
					finalForm = append(finalForm, v)
					break
				}
			}
		}

		if strings.Contains(v.Title, "居家观察") {
			finalForm = append(finalForm, v)
			continue
		}

		if strings.Contains(v.Title, "符合的场景") {
			for kk, vv := range v.FieldItems {
				if strings.Contains(vv.Content, "以上都不符合") {
					v.Value = v.FieldItems[kk].Content
					v.FieldItems = v.FieldItems[kk : kk+1]
					finalForm = append(finalForm, v)
					break
				}
			}
		}
	}

	form.Address = envMap[EnvKeyAddress]
	form.Form = finalForm

	formStr, _ := json.Marshal(form)
	log.Printf("RenderForm 结果：%s\n", formStr)
}

func submit() {
	var (
		err     error
		rawData []byte
	)

	rawData, _, err = post(URLSubmitForm, form)
	if err != nil {
		log.Fatalf("Submit 请求失败：%s", err.Error())
	}

	log.Printf("Submit 结果：%s", string(rawData))
}

func get(url string) ([]byte, *http.Response, error) {
	return httpDo(http.MethodGet, url, nil)
}

func post(url string, data interface{}) ([]byte, *http.Response, error) {
	return httpDo(http.MethodPost, url, data)
}

func httpDo(method, url string, data interface{}) ([]byte, *http.Response, error) {
	var (
		err  error
		req  *http.Request
		resp *http.Response
		body []byte
		ret  []byte
	)

	if data != nil {
		body, _ = json.Marshal(data)
	}

	req, _ = http.NewRequest(method, renderURL(url), bytes.NewBuffer(body))

	req.Header.Set("User-Agent", envMap[EnvKeyUserAgent])
	req.Header.Set("Cookie", envMap[EnvKeyCookie])
	req.Header.Set("Cpdaily-Extension", envMap[EnvKeyExtension])

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP %s 失败：%w", method, err)
	}

	ret, err = ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()

	return ret, resp, err
}

func renderURL(url string) string {
	if strings.HasPrefix(url, "http") {
		return url
	}

	return envMap[EnvKeyDomain] + url
}
