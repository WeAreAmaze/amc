package evm

import (
	"github.com/amazechain/amc/cmd/evmsdk"
)

/*
sample:
设置

	{
		"type":"setting",
		"val":{
			"app_base_path":"/sd/0/data/evm",//文件存储基础路径
			"priv_key":"37d15846af852c47649005fd6dfe33483d394aaa60c14e7c56f4deadb116329e",//私钥
			"account":"0xeb156a42dcafcf155b07f3638892440c7de5d564",//账户
			"server_uri":    "ws://127.0.0.1:20013"//服务端地址
			"log_level":"",//传'debug'显示日志,传空串或其他值不显示日志。
		}
	}

启动

	{
		"type":"start"
	}

停止

	{
		"type":"stop"
	}

获取列表数据

	{
		"type":"list"
	}

bls签名

	{
		"type":"blssign",
		"val":{
			"priv_key":"AQMEBQYHCA==",
			"msg":""
		}
	}

	{
		"code":0,
		"message":"",
		"data":"0123456789ABCDEF"
	}

bls公钥

	{
		"type":"blspubk",
		"val":{
			"priv_key":"37d15846af852c47649005fd6dfe33483d394aaa60c14e7c56f4deadb116329e"
		}
	}

	{
		"code":0,
		"message":"",
		"data":"0123456789ABCDEF"
	}

引擎状态

	{
		"type":"state"
	}
	{
		"code":0,
		"message":"",
		"data":"started",//started,stopped
	}
*/
func Emit(jsonText string) string {
	return evmsdk.Emit(jsonText)
}
