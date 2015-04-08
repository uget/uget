package api

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Unknwon/macaron"
	log "github.com/cihub/seelog"
	"github.com/muja/uget/core"
	"github.com/robertkrimen/otto"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var crossdomain = `<?xml version="1.0"?>
<!DOCTYPE cross-domain-policy SYSTEM "http://www.macromedia.com/xml/dtds/cross-domain-policy.dtd">
<cross-domain-policy>
<allow-access-from domain="*" />
</cross-domain-policy>
`

type Server struct {
	BindAddr  string    `json:"bind_address,omitempty"`
	Port      uint16    `json:"port"`
	StartedAt time.Time `json:"started_at"`
}

var downloader = core.NewDownloader()

type macaronLog struct{}

func (w macaronLog) Write(p []byte) (int, error) {
	log.Info(strings.TrimSpace(string(p)))
	return len(p), nil
}

func (s *Server) Run() {
	m := macaron.NewWithLogger(macaronLog{})
	m.Use(macaron.Renderer())
	// JSON API
	m.Group("", func() {
		m.Get("/serverinfo", wrapJSON(s))
		m.Group("/containers", func() {
			m.Post("", s.createContainer)
			m.Get("", s.listContainers)
			m.Get("/:id", s.showContainer)
			m.Delete("/:id", s.deleteContainer)
		})
	})
	// CLICK'N'LOAD v2
	m.Group("", func() {
		m.Post("/flash/addcrypted2", clickNLoad)
		m.Get("/flash", wrap("UGET"))
		m.Get("/jdcheck.js", as("text/javascript"), wrap("jdownloader = true;"))
		m.Get("/crossdomain.xml", as("text/html"), wrap(crossdomain))
	})
	s.StartedAt = time.Now().Round(time.Minute)
	m.Run(s.BindAddr, int(s.Port))
}

func clickNLoad(r *http.Request) (int, string) {
	jk := r.FormValue("jk")
	// pw := r.FormValue("pw")
	crypted := r.FormValue("crypted")
	vm := otto.New()
	value, err1 := vm.Run(jk)
	value, err := vm.Run("f()")
	if err != nil || err1 != nil {
		return http.StatusBadRequest, "Invalid Javascript in query param 'jk'."
	}
	key, err := hex.DecodeString(value.String())
	if err != nil {
		return http.StatusBadRequest, "String returned from JS function was not valid HEX."
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return http.StatusBadRequest, "Invalid AES key."
	}
	data, err := base64.StdEncoding.DecodeString(crypted)
	if err != nil {
		return http.StatusBadRequest, "Invalid b64 encoded string in param 'crypted'."
	}
	c := cipher.NewCBCDecrypter(block, key)
	c.CryptBlocks(data, data)
	result := regexp.MustCompile(`\s+`).Split(string(data), -1)
	fmt.Printf("Added new container with %v links.\n", len(result))
	return http.StatusOK, "success\r\n"
}

func (s *Server) createContainer(c *macaron.Context) {
	var container struct {
		string `json:"p"`
	}
	decoder := json.NewDecoder(c.Req.Body().ReadCloser())
	if decoder.Decode(&container) != nil {
		c.Render.Error(http.StatusNotFound, "Invalid JSON.")
	}
	c.Render.RawData(http.StatusOK, []byte("okay!"))
}

func (s *Server) listContainers(c *macaron.Context) {

}

func (s *Server) showContainer(c *macaron.Context) {

}

func (s *Server) deleteContainer(c *macaron.Context) {
	fmt.Printf("Deleting %s\n", c.Params("id"))
	c.Status(http.StatusNoContent)
}

func as(ctype string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", fmt.Sprintf("%s; charset=utf-8", ctype))
	}
}

func wrap(v interface{}) func() interface{} {
	return func() interface{} {
		return v
	}
}

// Wraps a static value in a function block
// This is a convenience method to use with macaron
func wrapJSON(v interface{}) func(*macaron.Context) {
	return func(ctx *macaron.Context) {
		ctx.JSON(http.StatusOK, v)
	}
}
