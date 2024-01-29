package selenium

import "net/http"

const (
	newSession = "newSession"
	delSession = "delSession"
	getStatus  = "getStatus"
)

var apiMap = map[string]api{
	"newSession":          {http.MethodPost, "/session"},
	"delSession":          {http.MethodDelete, "/session/{session id}"},
	"getStatus":           {http.MethodGet, "/status"},
	"getTimeout":          {http.MethodGet, "/session/{session id}/timeouts"},
	"setTimeout":          {http.MethodPost, "/session/{session id}/timeouts"},
	"openUrl":             {http.MethodPost, "/session/{session id}/url"},
	"getUrl":              {http.MethodGet, "/session/{session id}/url"},
	"back":                {http.MethodPost, "/session/{session id}/back"},
	"forward":             {http.MethodPost, "/session/{session id}/forward"},
	"refresh":             {http.MethodPost, "/session/{session id}/refresh"},
	"getTitle":            {http.MethodGet, "/session/{session id}/title"},
	"getWindowHandle":     {http.MethodGet, "/session/{session id}/window"},
	"closeWindow":         {http.MethodDelete, "/session/{session id}/window"},
	"switchToWindow":      {http.MethodPost, "/session/{session id}/window"},
	"getWindowHandles":    {http.MethodGet, "/session/{session id}/window/handles"},
	"newWindow":           {http.MethodPost, "/session/{session id}/window/new"},
	"switchToFrame":       {http.MethodPost, "/session/{session id}/frame"},
	"switchToParentFrame": {http.MethodPost, "/session/{session id}/frame/parent"},
	"getWindowRect":       {http.MethodGet, "/session/{session id}/window/rect"},
	"setWindowRect":       {http.MethodPost, "/session/{session id}/window/rect"},
	"maximizeWindow":      {http.MethodPost, "/session/{session id}/window/maximize"},
	"minimizeWindow":      {http.MethodPost, "/session/{session id}/window/minimize"},
	"fullscreenWindow":    {http.MethodPost, "/session/{session id}/window/fullscreen"},
}

type api struct {
	Method string
	Path   string
}
