// +build clickandload

package server

import (
	"fmt"

	"github.com/Unknwon/macaron"
	"github.com/uget/cnlv2"
	"github.com/uget/uget/core"
)

func cnl(m *macaron.Macaron) {
	m.Group("", func() {
		m.Post("/flash/addcrypted2", cnlv2.HttpAction(addLinks, nil))
		m.Get("/flash", wrap(fmt.Sprintf("uget %s", core.Version)))
		m.Get("/jdcheck.js", as("text/javascript"), wrap("jdownloader = true;"))
		m.Get("/crossdomain.xml", as("text/html"), wrap(cnlv2.CrossDomain))
	})
}
