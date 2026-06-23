package endpoints

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/prebid/prebid-server/v3/util/uuidutil"
)

// NewUserSyncRedirectEndpoint returns a handler that issues a 302 redirect to the URL
// provided via the `redirect` query parameter. It accepts the following query params:
// gdpr, gdpr_consent, us_privacy, gpp, gpp_sid and redirect. Only `redirect` is used.
// Additionally, it will replace the [TVUID] macro in the redirect URL with the value
// of the 'tvid' cookie if it exists. If the cookie does not exist, it will generate
// a UUID, replace the macro with it, and set the 'tvid' cookie in the response.
func NewUserSyncRedirectEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		q := r.URL.Query()
		redir := q.Get("redirect")
		if strings.TrimSpace(redir) == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing redirect parameter"))
			return
		}

		// Resolve TVUID value from cookie or generate a new one
		var tvuid string
		if c, err := r.Cookie("tvid"); err == nil && c != nil && c.Value != "" {
			tvuid = c.Value
		} else {
			// generate new UUID
			id, err := (uuidutil.UUIDRandomGenerator{}).Generate()
			if err == nil && id != "" {
				tvuid = id
				// set cookie in response
				http.SetCookie(w, &http.Cookie{
					Name:  "tvid",
					Value: tvuid,
					Path:  "/",
					//HttpOnly: true,
					MaxAge:   7776000,
					Secure:   true,
					SameSite: http.SameSiteNoneMode,
				})
			}
		}

		// Replace [TVUID] macro if present
		if tvuid != "" && strings.Contains(redir, "[TVUID]") {
			redir = strings.ReplaceAll(redir, "[TVUID]", tvuid)
		}

		// Basic header injection hardening: strip CR/LF to avoid response splitting
		redir = strings.ReplaceAll(redir, "\r", "")
		redir = strings.ReplaceAll(redir, "\n", "")

		w.Header().Set("Location", redir)
		w.WriteHeader(http.StatusFound)
	}
}
