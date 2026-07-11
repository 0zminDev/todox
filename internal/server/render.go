package server

import "net/http"

func render(w http.ResponseWriter, name string, data any) {
	if err := tpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
