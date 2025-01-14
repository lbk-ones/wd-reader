package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
	"wd-reader/go/constant"
	"wd-reader/go/log"
)

var (
	once = sync.Once{}
	//flashHost  = "http://129.226.150.139:10000"
	flashHost  = constant.WD_SERVER + "/wd/parse"
	httpClient *http.Client

	connTimeout         = 2
	rwTimeout           = 1000
	maxIdleConns        = 100
	maxIdleConnsPerHost = 2
	idleConnTimeout     = time.Duration(180) * time.Second
)

func initHttpClient() {
	once.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:       time.Duration(connTimeout) * time.Second,
				KeepAlive:     time.Duration(rwTimeout*10) * time.Second,
				FallbackDelay: -1,
			}).DialContext,
			MaxIdleConns:          maxIdleConns,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   time.Duration(connTimeout) * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = new(http.Client)
		httpClient.Transport = transport
		httpClient.Timeout = time.Duration(rwTimeout) * time.Second
	})
}

// PostOctetStream 文件上传 加参数
func PostOctetStream(params map[string]string, data []byte) (*DataParse, error) {
	initHttpClient()
	u, err := url.Parse(flashHost)
	if err != nil {
		log.Logger.Errorf("Error parsing URL:,%v", err)
	}
	query := u.Query()
	for key, value := range params {
		query.Add(key, value)
	}
	u.RawQuery = query.Encode()
	s := u.String()

	log.Logger.Info("download url：" + s)
	httpReq, err := http.NewRequest("POST", s, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed create http request, error: %s", err.Error())
	}
	headers := make(map[string]string)
	headers["Content-Type"] = "application/octet-stream"
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed do request, error: %s", err.Error())
	}
	defer httpResp.Body.Close()
	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed read body, error: %s", err.Error())
	}
	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("http code not 200, respData: %s", string(respData))
	}
	resp := &DataParse{}
	err = json.Unmarshal(respData, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshal, respData: %s, error: %s", respData, err.Error())
	}
	if resp.Success != true {
		return resp, fmt.Errorf("request_id: %s, code: %d, message: %s", "ww", resp.Code, resp.Message)
	}
	return resp, nil

}
