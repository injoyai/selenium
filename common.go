package selenium

import (
	"github.com/injoyai/logs"
	"net/http"
	"net/url"
)

const (
	// DefaultURLPrefix is the default HTTP endpoint that offers the WebDriver API.
	DefaultURLPrefix = "http://127.0.0.1:4444/wd/hub"

	// jsonContentType is JSON content type.
	jsonContentType = "application/json"
)

// HTTPClient is the default client to use to communicate with the WebDriver
// server.
var HTTPClient = http.DefaultClient

func Debug(b ...bool) {
	if len(b) == 0 || b[0] {
		logs.SetLevel(logs.LevelAll)
	} else {
		logs.SetLevel(logs.LevelNone)
	}
}

// filteredURL replaces existing password from the given URL.
func filteredURL(u string) string {
	// Hide password if set in URL
	m, err := url.Parse(u)
	if err != nil {
		return ""
	}
	if m.User != nil {
		if _, ok := m.User.Password(); ok {
			m.User = url.UserPassword(m.User.Username(), "__password__")
		}
	}
	return m.String()
}
