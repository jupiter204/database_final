package main
import (
	"fmt"
	"net/http"
)
func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "後端測試中：Go API 運作正常！")
	})
	http.ListenAndServe(":8080", nil)
}