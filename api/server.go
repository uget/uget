package api

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/Unknwon/macaron"
	"github.com/robertkrimen/otto"
	"net/http"
	"regexp"
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

func (s *Server) Run() {
	m := macaron.Classic()
	m.Use(macaron.Renderer())
	// JSON API
	m.Group("", func() {
		m.Get("/serverinfo", wrapJSON(s))
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
		return 400, "Invalid Javascript in query param 'jk'."
	}
	key, err := hex.DecodeString(value.String())
	if err != nil {
		return 400, "String returned from JS function was not valid HEX."
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return 400, "Invalid AES key."
	}
	data, err := base64.StdEncoding.DecodeString(crypted)
	if err != nil {
		return 400, "Invalid b64 encoded string in param 'crypted'."
	}
	c := cipher.NewCBCDecrypter(block, key)
	c.CryptBlocks(data, data)
	result := regexp.MustCompile(`\s+`).Split(string(data), -1)
	fmt.Printf("Added new container with %v links.\n", len(result))
	return 200, "success\r\n"
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
		ctx.JSON(200, v)
	}
}
