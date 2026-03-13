package es

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	"github.com/elastic/go-elasticsearch/v9"
)

var (
	esClient *elasticsearch.Client
	once     sync.Once
)

func GetESClient(url string, username, password string) *elasticsearch.Client {
	once.Do(func() {
		var err error
		config := elasticsearch.Config{
			Addresses: []string{url},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 跳过证书验证
			},
		}

		// 添加认证配置
		if username != "" && password != "" {
			config.Username = username
			config.Password = password
		}

		esClient, err = elasticsearch.NewClient(config)
		if err != nil {
			fmt.Printf("elasticsearch.NewClient failed, err:%v\n", err)
		}
	})
	return esClient
}
