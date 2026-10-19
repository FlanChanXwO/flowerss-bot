package bot

import (
 "bytes"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strings"
 "time"

 "github.com/indes/flowerss-bot/internal/model"
 tb "gopkg.in/telebot.v3"
)

const maxRSSMediaBytes = 48 << 20

var rssMediaClient = &http.Client{Timeout: 45*time.Second}

func isMediaURLFetchError(err error) bool {
 return err != nil && (strings.Contains(err.Error(), "failed to get HTTP URL content (400)") ||
  strings.Contains(err.Error(), "wrong HTTP URL specified (400)") ||
  strings.Contains(err.Error(), "failed to upload file by URL (400)"))
}

// downloadRSSMedia fetches media only after Telegram rejects the original URL.
// Validate content and size before uploading it rather than uploading an HTTP error page.
func downloadRSSMedia(item model.Media, caption string) (interface{}, error) {
 parsed, err := url.Parse(item.URL)
 if err != nil || (parsed.Scheme!="https" && parsed.Scheme!="http") || parsed.Host=="" || parsed.User!=nil {
  return nil, fmt.Errorf("invalid RSS media URL")
 }
 req,err:=http.NewRequest(http.MethodGet,item.URL,nil)
 if err!=nil {return nil,err}
 resp,err:=rssMediaClient.Do(req)
 if err!=nil {return nil,err}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK {return nil,fmt.Errorf("media origin returned HTTP %d",resp.StatusCode)}
 if resp.ContentLength>maxRSSMediaBytes {return nil,fmt.Errorf("media exceeds %d bytes",maxRSSMediaBytes)}
 data,err:=io.ReadAll(io.LimitReader(resp.Body,maxRSSMediaBytes+1))
 if err!=nil {return nil,err}
 if len(data)>maxRSSMediaBytes {return nil,fmt.Errorf("media exceeds %d bytes",maxRSSMediaBytes)}
 ct:=http.DetectContentType(data)
 file:=tb.FromReader(bytes.NewReader(data))
 switch item.Kind {
 case model.MediaPhoto:
  // Telegram photos accept JPEG/PNG; WebP needs conversion to JPEG.
  if strings.HasPrefix(ct,"image/webp") {
   return &tb.Document{File:file,Caption:caption,FileName:"rss-image.webp",MIME:"image/webp"},nil
  } else if !strings.HasPrefix(ct,"image/jpeg") && !strings.HasPrefix(ct,"image/png") {
   return nil,fmt.Errorf("unsupported photo content type %s",ct)
  }
  return &tb.Photo{File:file,Caption:caption},nil
 case model.MediaVideo:
  if !strings.HasPrefix(ct,"video/mp4") && !bytes.Contains(data[:mediaPrefixLength(len(data))],[]byte("ftyp")) {return nil,fmt.Errorf("unsupported video content type %s",ct)}
  return &tb.Video{File:file,Caption:caption,Streaming:true,FileName:"rss-video.mp4"},nil
 case model.MediaAnimation:
  if !strings.HasPrefix(ct,"image/gif") && !bytes.Contains(data[:mediaPrefixLength(len(data))],[]byte("ftyp")) {return nil,fmt.Errorf("unsupported animation content type %s",ct)}
  return &tb.Animation{File:file,Caption:caption,FileName:"rss-animation.gif"},nil
 case model.MediaAudio:
  return &tb.Audio{File:file,Caption:caption,FileName:"rss-audio.mp3"},nil
 }
 return nil,fmt.Errorf("unsupported media kind %s",item.Kind)
}

func mediaPrefixLength(n int) int { if n<32{return n};return 32 }
