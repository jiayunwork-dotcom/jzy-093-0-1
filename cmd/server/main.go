// Command server 启动常驻的反渗透核算 HTTP 服务，固定监听 8080 端口。
package main

import (
	"log"
	"net/http"

	"rocalc/internal/api"
	"rocalc/internal/casefile"
)

const addr = ":8080"

func main() {
	srv := api.New(casefile.DefaultRegistry())
	log.Printf("反渗透核算服务已启动，监听 %s，内置工况档：%v", addr, casefile.DefaultRegistry().Names())
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("服务退出：%v", err)
	}
}
