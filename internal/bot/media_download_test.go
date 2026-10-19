package bot

import (
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/indes/flowerss-bot/internal/model"
 tb "gopkg.in/telebot.v3"
)

func TestMediaFallbackAfterTelegramURL400(t *testing.T) {
 media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  w.Header().Set("Content-Type","image/jpeg")
  w.Write([]byte{0xff,0xd8,0xff,0xe0,0,0x10,'J','F','I','F',0,1,1,0,0,1,0,1,0,0})
 }))
 defer media.Close()
 var methods []string
 api:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  methods=append(methods,r.Header.Get("Content-Type"))
  if len(methods)==1 {
   w.WriteHeader(http.StatusBadRequest)
   fmt.Fprint(w,`{"ok":false,"error_code":400,"description":"Bad Request: failed to get HTTP URL content"}`)
   return
  }
  if !strings.Contains(r.Header.Get("Content-Type"),"multipart/form-data") {t.Errorf("fallback must upload bytes")}
  fmt.Fprint(w,`{"ok":true,"result":{"message_id":1,"photo":[{"file_id":"test"}]}}`)
 }))
 defer api.Close()
 tg,err:=tb.NewBot(tb.Settings{URL:api.URL,Token:"TEST",Client:api.Client(),Offline:true})
 if err!=nil {t.Fatal(err)}
 b:=&Bot{tb:tg}
 sent,err:=b.sendNewsMedia(&tb.Chat{ID:123},&tb.SendOptions{ThreadID:7},[]model.Media{{Kind:model.MediaPhoto,URL:media.URL+"/img.jpg"}},"caption")
 if err!=nil||!sent {t.Fatalf("sent=%v err=%v",sent,err)}
 if len(methods)!=2 {t.Fatalf("telegram sends=%d want 2",len(methods))}
}

func TestFailedMediaOriginReturnsError(t *testing.T) {
 media:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {w.WriteHeader(422);fmt.Fprint(w,"Pixiv animation is not a static image")}))
 defer media.Close()
 _,err:=downloadRSSMedia(model.Media{URL:media.URL,Kind:model.MediaPhoto},"")
 if err==nil||!strings.Contains(err.Error(),"HTTP 422"){t.Fatalf("expected origin error: %v",err)}
}
