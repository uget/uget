package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/Unknwon/macaron"
	"github.com/uget/uget/app"
	"github.com/uget/uget/core"
)

// Server listens for HTTP requests that manipulate files
type Server struct {
	BindAddr  string    `json:"bind_address,omitempty"`
	Port      uint16    `json:"port"`
	StartedAt time.Time `json:"started_at"`
	client    *core.Client
	downloads *sync.Map
}

// On initializes the server at the given address
func On(bind string, port uint16) *Server {
	return &Server{
		BindAddr:  bind,
		Port:      port,
		StartedAt: time.Now(),
		client:    core.NewClientWith(0),
		downloads: new(sync.Map),
	}
}

type macaronLog struct{}

func (w macaronLog) Write(p []byte) (int, error) {
	logrus.Info(strings.TrimSpace(string(p)))
	return len(p), nil
}

// Use adds account to this server. passes to its internal core.Client#Use method
func (s *Server) Use(acc core.Account) {
	s.client.Use(acc)
}

// Run starts the server
func (s *Server) Run() {
	m := macaron.NewWithLogger(macaronLog{})
	m.Use(macaron.Renderer())
	m.Use(func(m *macaron.Context) {
		host, _, err := net.SplitHostPort(m.Req.RemoteAddr)
		if err != nil {
			return
		}
		if !net.ParseIP(host).IsLoopback() {
			m.Render.Error(http.StatusForbidden, "only local requests are allowed")
		}
		m.Header().Set("Access-Control-Allow-Origin", "*")
		m.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE")
		m.Header().Set("Access-Control-Allow-Headers", "origin, x-requested-with, content-type")
	})
	// JSON API
	m.Get("/", as("text/html"), wrap(`<!DOCTYPE html>
<html>
<head>
<script type="text/javascript" src="https://unpkg.com/react/umd/react.development.js"></script>
<script type="text/javascript" src="https://unpkg.com/react-dom/umd/react-dom.development.js"></script>
<script type="text/javascript">
	ReactDOM.render(
		<h1>Hello, world!</h1>,
		document.getElementById('root')
	);
</script>
</head>
<body>
</body>
</html>`))
	m.Group("", func() {
		m.Get("/serverinfo", wrapJSON(s))
		m.Group("/containers", func() {
			m.Post("", s.createContainer)
			m.Get("", s.listContainers)
			m.Get("/:id", s.showContainer)
			m.Delete("/:id", s.deleteContainer)
			m.Options("/:id", func() {})
		})
		m.Group("/accounts", func() {
			m.Get("", s.listAccounts)
		})
	})
	// CLICK'N'LOAD v2
	// m.Group("", func() {
	// 	m.Post("/flash/addcrypted2", cnlv2.HttpAction(s.addLinks, nil))
	// 	m.Get("/flash", wrap(fmt.Sprintf("uget %s", core.Version)))
	// 	m.Get("/jdcheck.js", as("text/javascript"), wrap("jdownloader = true;"))
	// 	m.Get("/crossdomain.xml", as("text/html"), wrap(cnlv2.CrossDomain))
	// })
	s.cnl(m)
	s.client.OnDownload(func(d *core.Download) {
		s.downloads.Store(d.File.ID(), d)
		d.Wait()
		s.downloads.Delete(d.File.ID())
	})
	s.client.Start()
	m.Run(s.BindAddr, int(s.Port))
}

func (s *Server) listAccounts(c *macaron.Context) {
	fmt.Println("list accounts")
	accs := []map[string]interface{}{}
	for _, provider := range core.RegisteredProviders() {
		if p, ok := provider.(core.Accountant); ok {
			for _, meta := range app.AccountManagerFor("", p).Metadata() {
				m := map[string]interface{}{
					"id":       meta.Data.ID(),
					"disabled": meta.Disabled,
					"provider": meta.Provider,
					"data":     meta.Data,
				}
				accs = append(accs, m)
			}
		}
	}
	c.Render.JSON(200, accs)
}

func (s *Server) addLinks(links []string) {
	logrus.Debugf("Added %v links!", len(links))
}

func (s *Server) createContainer(c *macaron.Context) {
	var container []string
	decoder := json.NewDecoder(c.Req.Body().ReadCloser())
	if err := decoder.Decode(&container); err != nil {
		c.Render.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Error: %v.", err))
	} else {
		urls := make([]*url.URL, len(container))
		for i, u := range container {
			urls[i], err = url.Parse(u)
			if err != nil {
				c.Render.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid URL #%d: %v.", i+1, err))
				return
			}
		}
		container := s.client.AddURLs(urls)
		c.Render.JSON(http.StatusOK, map[string]string{"id": container.ID().String()})
	}
}

func (s *Server) listContainers(c *macaron.Context) {
	c.Render.JSON(http.StatusOK, s.client.ResolvedQueue.List())
}

func (s *Server) showContainer(c *macaron.Context) {}

func (s *Server) deleteContainer(c *macaron.Context) {
	id := c.Params("id")
	if len(id) < 4 {
		c.Render.Error(http.StatusInternalServerError, fmt.Sprintf("invalid ID: %v", id))
	} else if len(id) == 64 { // exact id
		s.client.ResolvedQueue.Remove(id)
	} else {
		for _, file := range s.client.ResolvedQueue.List() {
			if strings.HasPrefix(file.ID(), id) {
				s.client.ResolvedQueue.Remove(file.ID())
				c.Render.JSON(http.StatusOK, file)
				return
			}
		}
		c.Render.Error(http.StatusNotFound, "ID does not match any files!")
	}
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
