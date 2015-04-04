package api

import (
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/encoder"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	BindAddr  string    `json:"address,omitempty"`
	Port      uint16    `json:"port"`
	StartedAt time.Time `json:"started_at"`
}

func (s *Server) Run() {
	m := martini.New()
	r := martini.NewRouter()
	m.Action(r.Handle)
	m.Use(jsonMiddleware)
	r.Get("/serverinfo", encode(s))
	s.StartedAt = time.Now().Round(time.Minute)
	m.RunOnAddr(s.BindAddr + ":" + fmt.Sprintf("%v", s.Port))
}

func jsonMiddleware(c martini.Context, w http.ResponseWriter, r *http.Request) {
	pretty, _ := strconv.ParseBool(r.FormValue("pretty_json"))
	ctype := r.Header.Get("Content-Type")
	switch ctype {
	default:
		c.MapTo(encoder.JsonEncoder{PrettyPrint: pretty}, (*encoder.Encoder)(nil))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
}

func encode(v interface{}) func(encoder.Encoder) []byte {
	return func(enc encoder.Encoder) []byte {
		return encoder.Must(enc.Encode(v))
	}
}
