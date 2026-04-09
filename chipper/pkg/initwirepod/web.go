package initwirepod

import (
	"fmt"
	"net/http"
)

// cant be part of config-ws, otherwise import cycle

func ChipperHTTPApi(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api-chipper/restart":
		RestartServer()
		fmt.Fprint(w, "done")
		return
	}
}
