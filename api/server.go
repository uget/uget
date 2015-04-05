package api

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/encoder"
	"github.com/robertkrimen/otto"
	"net/http"
	"regexp"
	"strconv"
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
	m := martini.New()
	r := martini.NewRouter()
	m.Action(r.Handle)
	m.Use(martini.Logger())
	// API
	r.Group("", func(r martini.Router) {
		r.Get("/serverinfo", wrapEncode(s))
	}, jsonMiddleware)
	// CLICK'N'LOAD v2
	r.Group("", func(r martini.Router) {
		r.Post("/flash/addcrypted2", clickNLoad)
		r.Get("/flash", wrap("UGET"))
		r.Get("/jdcheck.js", as("text/javascript"), wrap("jdownloader = true;"))
		r.Get("/crossdomain.xml", as("text/html"), wrap(crossdomain))
	})
	s.StartedAt = time.Now().Round(time.Minute)
	m.RunOnAddr(fmt.Sprintf("%v:%v", s.BindAddr, s.Port))
}

func clickNLoad(r *http.Request) (int, string) {
	jk := r.FormValue("jk")
	// pw := r.FormValue("pw")
	crypted := r.FormValue("crypted")
	vm := otto.New()
	vm.Run(jk)
	value, _ := vm.Run("f()")
	key, _ := hex.DecodeString(value.String())
	block, _ := aes.NewCipher(key)
	data, _ := base64.StdEncoding.DecodeString(crypted)
	c := cipher.NewCBCDecrypter(block, key)
	c.CryptBlocks(data, data)
	result := regexp.MustCompile(`\s+`).Split(string(data), -1)
	fmt.Printf("Added new container with %v links.\n", len(result))
	return 200, "success\r\n"
}

func jsonMiddleware(c martini.Context, w http.ResponseWriter, r *http.Request) {
	pretty, _ := strconv.ParseBool(r.FormValue("pretty_json"))
	c.MapTo(encoder.JsonEncoder{PrettyPrint: pretty}, (*encoder.Encoder)(nil))
	as("application/json")(w)
}

func as(ctype string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", fmt.Sprintf("%s; charset=utf-8", ctype))
	}
}

func wrap(v string) func() string {
	return func() string {
		fmt.Print("EXIT")
		return v
	}
}

// Wraps a static value in a function block
// This is a convenience method to use with martini
func wrapEncode(v interface{}) func(encoder.Encoder) []byte {
	return func(enc encoder.Encoder) []byte {
		return encoder.Must(enc.Encode(v))
	}
}
