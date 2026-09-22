// Package httputil 提供 HTTP 响应写出的小工具。
package httputil

import (
	"encoding/json"
	"net/http"
)

// WriteXML 以 text/xml 写出响应。
func WriteXML(w http.ResponseWriter, xml string) {
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write([]byte(xml))
}

// WriteJSON 以 application/json 写出响应。
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
