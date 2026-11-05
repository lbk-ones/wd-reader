package server

// Data 返回实体
type Data struct {
	Error     int         `json:"error"`
	StartTime int64       `json:"startTime"`
	EndTime   int64       `json:"endTime"`
	Code      string      `json:"code"`
	RpcMethod interface{} `json:"rpcMethod"`
	Message   string      `json:"message"`
	ErrorInfo interface{} `json:"errorInfo"`
	Data      string      `json:"data"`
	Success   bool        `json:"success"`
}

type DataParse struct {
	Error     int         `json:"error"`
	StartTime int64       `json:"startTime"`
	EndTime   int64       `json:"endTime"`
	Code      string      `json:"code"`
	RpcMethod interface{} `json:"rpcMethod"`
	Message   string      `json:"message"`
	ErrorInfo interface{} `json:"errorInfo"`
	Data      struct {
		ExeStatus   string `json:"exeStatus"`
		ExeLog      string `json:"exeLog"`
		DownloadUrl string `json:"downloadUrl"`
		FileUrl     string `json:"fileUrl"`
		OriginName  string `json:"originName"`
	} `json:"data"`
	Success bool `json:"success"`
}
