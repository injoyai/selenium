package selenium

import (
	"github.com/fatih/color"
	"github.com/injoyai/conv"
	"github.com/injoyai/logs"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultURLPrefix is the default HTTP endpoint that offers the WebDriver API.
	DefaultURLPrefix = "http://127.0.0.1:4444/wd/hub"

	// jsonContentType is JSON content type.
	jsonContentType = "application/json"

	// DefaultWaitInterval is the default polling interval for selenium.Wait
	// function.
	DefaultWaitInterval = 100 * time.Millisecond

	// DefaultWaitTimeout is the default timeout for selenium.Wait function.
	DefaultWaitTimeout = 60 * time.Second
)

// HTTPClient is the default client to use to communicate with the WebDriver
// server.
var HTTPClient = &http.Client{}

func init() {
	logs.DefaultWrite.SetFormatter(logs.TimeFormatter).SetColor(color.FgMagenta)
	logs.DefaultRead.SetFormatter(logs.TimeFormatter).SetColor(color.FgMagenta)
}

// Debug 设置调试模式,打印日志
func Debug(b ...bool) {
	level := conv.Select(len(b) == 0 || b[0], logs.LevelAll, logs.LevelNone)
	logs.SetLevel(level.(logs.Level))
}

// SetLogLevel 设置日志等级
func SetLogLevel(level logs.Level) {
	logs.SetLevel(level)
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
