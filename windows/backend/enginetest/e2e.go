package main
import (
 "fmt";"io";"net/http";"net/url";"os";"time"
 "snowden-system/backend/core"
)
type h struct{}
func (h) OnLog(s string) { fmt.Println("LOG:",s[:min(100,len(s))]) }
func min(a,b int)int{if a<b{return a};return b}
func main(){
 cfg,_:=os.ReadFile("build/bin/assets/configs/test-reality-20810.json")
 e:=core.NewEngine()
 e.SetLogHandler(h{})
 if err:=e.Start(cfg);err!=nil{fmt.Println("START FAILED:",err);return}
 defer e.Close()
 fmt.Println("running, probing (3 proto urltest)...")
 time.Sleep(5*time.Second)
 pu,_:=url.Parse("http://127.0.0.1:20810")
 cl:=&http.Client{Timeout:20*time.Second,Transport:&http.Transport{Proxy:http.ProxyURL(pu)}}
 // Speed test: 10MB download
 fmt.Println("downloading 10MB...")
 r,err:=cl.Get("https://speed.cloudflare.com/__down?bytes=10000000")
 if err!=nil{fmt.Println("FAIL:",err);return}
 b,_:=io.ReadAll(r.Body);r.Body.Close()
 fmt.Printf("speed: %.1f KB/s (%d bytes in ?s)\n",float64(len(b))/1024, len(b))
 // t.me
 r2,err:=cl.Get("https://t.me")
 if err!=nil{fmt.Println("T.ME FAIL:",err);return}
 r2.Body.Close()
 fmt.Printf("t.me: %d\n",r2.StatusCode)
}
