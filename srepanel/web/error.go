package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/csrf"
)

type ErrorState struct {
	*State
	Message    string
	SuperAdmin bool
}

func (s *Server) renderError(wr http.ResponseWriter, req *http.Request, status int, message string, state *State) {
	wr.WriteHeader(status)
	estate := ErrorState{State: state, Message: message, SuperAdmin: false}

	// Inject CSRF token
	type TemplateData struct {
		Data      interface{}
		CSRFField template.HTML
		CSRFToken string
	}

	tmplData := TemplateData{
		Data:      estate,
		CSRFField: csrf.TemplateField(req),
		CSRFToken: csrf.Token(req),
	}

	if err := errorPage.Execute(wr, tmplData); err != nil {
		log.Print("Failed to render error page:", err)
		_, _ = wr.Write([]byte("Failed to render the error page"))
	}
}
