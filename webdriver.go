// Remote Selenium client implementation.
// See https://www.w3.org/TR/webdriver for the protocol.

package selenium

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blang/semver"
	"github.com/injoyai/conv"
	"github.com/injoyai/goutil/g"
	"github.com/injoyai/goutil/oss"
	"github.com/injoyai/logs"
	"github.com/injoyai/selenium/firefox"
	"github.com/injoyai/selenium/log"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Errors returned by Selenium server.
var remoteErrors = map[int]string{
	6:  "invalid session ID",
	7:  "no such element",
	8:  "no such frame",
	9:  "unknown command",
	10: "stale element reference",
	11: "element not visible",
	12: "invalid element state",
	13: "unknown error",
	15: "element is not selectable",
	17: "javascript error",
	19: "xpath lookup error",
	21: "timeout",
	23: "no such window",
	24: "invalid cookie domain",
	25: "unable to set cookie",
	26: "unexpected alert open",
	27: "no alert open",
	28: "script timeout",
	29: "invalid element coordinates",
	32: "invalid selector",
}

type WebDriver struct {
	id, urlPrefix string
	capabilities  Capabilities
	w3cCompatible bool
	// storedActions stores KeyActions and PointerActions for later execution.
	storedActions  Actions
	browser        string
	browserVersion semver.Version

	wait
}

// Copy 复制实例,可以生成多个标签页的实例
func (this *WebDriver) Copy() *WebDriver {
	x := *this
	return &x
}

// SessionID returns the current session ID
func (wd *WebDriver) SessionID() string {
	return wd.id
}

func newRequest(method string, url string, data []byte) (*http.Request, error) {
	request, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Accept", jsonContentType)
	return request, nil
}

func (wd *WebDriver) requestURL(template string, args ...interface{}) string {
	return wd.urlPrefix + fmt.Sprintf(template, args...)
}

// TODO(minusnine): provide a "sessionURL" function that prepends the
// /session/<id> URL prefix and replace most requestURL (and voidCommand) calls
// with it.

type serverReply struct {
	SessionID *string // SessionID can be nil.
	Value     json.RawMessage

	// The following fields were used prior to Selenium 3.0 for error state and
	// in ChromeDriver for additional information.
	Status int
	State  string

	Error
}

// Error contains information about a failure of a command. See the table of
// these strings at https://www.w3.org/TR/webdriver/#handling-errors .
//
// This error type is only returned by servers that implement the W3C
// specification.
type Error struct {
	// Err contains a general error string provided by the server.
	Err string `json:"error"`
	// Message is a detailed, human-readable message specific to the failure.
	Message string `json:"message"`
	// Stacktrace may contain the server-side stacktrace where the error occurred.
	Stacktrace string `json:"stacktrace"`
	// HTTPCode is the HTTP status code returned by the server.
	HTTPCode int
	// LegacyCode is the "Response Status Code" defined in the legacy Selenium
	// WebDriver JSON wire protocol. This code is only produced by older
	// Selenium WebDriver versions, Chromedriver, and InternetExplorerDriver.
	LegacyCode int
}

// TODO(minusnine): Make Stacktrace more descriptive. Selenium emits a list of
// objects that enumerate various fields. This is not standard, though.

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Err, e.Message)
}

// execute performs an HTTP request and inspects the returned data for an error
// encoded by the remote end in a JSON structure. If no error is present, the
// entire, raw request payload is returned.
func (wd *WebDriver) execute(method, url string, data []byte) (json.RawMessage, error) {
	return executeCommand(method, url, data)
}

func executeCommand(method, url string, data []byte) (json.RawMessage, error) {
	logs.Writef(">>> %s %s %s\n", method, filteredURL(url), string(data))
	request, err := newRequest(method, url, data)
	if err != nil {
		return nil, err
	}

	response, err := HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	buf, err := io.ReadAll(response.Body)
	{
		//if err == nil {
		//	// Pretty print the JSON response
		//	var prettyBuf bytes.Buffer
		//	if err = json.Indent(&prettyBuf, buf, "", "    "); err == nil && prettyBuf.Len() > 0 {
		//		buf = prettyBuf.Bytes()
		//	}
		//}
		//response.Header["Content-Type"],
		logs.Readf("<<< %s %s\n", response.Status, string(buf))
	}
	if err != nil {
		return nil, errors.New(response.Status)
	}

	fullCType := response.Header.Get("Content-Type")
	cType, _, err := mime.ParseMediaType(fullCType)
	if err != nil {
		return nil, fmt.Errorf("got content type header %q, expected %q", fullCType, jsonContentType)
	}
	if cType != jsonContentType {
		return nil, fmt.Errorf("got content type %q, expected %q", cType, jsonContentType)
	}

	reply := new(serverReply)
	if err := json.Unmarshal(buf, reply); err != nil {
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad server reply status: %s", response.Status)
		}
		return nil, err
	}
	if reply.Err != "" {
		return nil, &reply.Error
	}

	// Handle the W3C-compliant error format. In the W3C spec, the error is
	// embedded in the 'value' field.
	if len(reply.Value) > 0 {
		respErr := new(Error)
		if err := json.Unmarshal(reply.Value, respErr); err == nil && respErr.Err != "" {
			respErr.HTTPCode = response.StatusCode
			return nil, respErr
		}
	}

	// Handle the legacy error format.
	const success = 0
	if reply.Status != success {
		shortMsg, ok := remoteErrors[reply.Status]
		if !ok {
			shortMsg = fmt.Sprintf("unknown error - %d", reply.Status)
		}

		longMsg := new(struct {
			Message string
		})
		if err := json.Unmarshal(reply.Value, longMsg); err != nil {
			return nil, errors.New(shortMsg)
		}
		return nil, &Error{
			Err:        shortMsg,
			Message:    longMsg.Message,
			HTTPCode:   response.StatusCode,
			LegacyCode: reply.Status,
		}
	}

	return buf, nil
}

// NewRemote creates new remote client, this will also start a new session.
// capabilities provides the desired capabilities. urlPrefix is the URL to the
// Selenium server, must be prefixed with protocol (http, https, ...).
//
// Providing an empty string for urlPrefix causes the DefaultURLPrefix to be
// used.
func NewRemote(capabilities Capabilities, urlPrefix string) (*WebDriver, error) {
	if urlPrefix == "" {
		urlPrefix = DefaultURLPrefix
	}

	wd := &WebDriver{
		urlPrefix:    urlPrefix,
		capabilities: capabilities,
	}
	if b := capabilities["browserName"]; b != nil {
		wd.browser = b.(string)
	}

	if _, err := wd.NewSession(); err != nil {
		return nil, err
	}
	return wd, nil
}

// DeleteSession deletes an existing session at the WebDriver instance
// specified by the urlPrefix and the session ID.
func DeleteSession(urlPrefix, id string) error {
	u, err := url.Parse(urlPrefix)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, "session", id)
	return voidCommand("DELETE", u.String(), nil)
}

func (wd *WebDriver) stringCommand(urlTemplate string) (string, error) {
	url := wd.requestURL(urlTemplate, wd.id)
	response, err := wd.execute("GET", url, nil)
	if err != nil {
		return "", err
	}

	reply := new(struct{ Value *string })
	if err := json.Unmarshal(response, reply); err != nil {
		return "", err
	}

	if reply.Value == nil {
		return "", fmt.Errorf("nil return value")
	}

	return *reply.Value, nil
}

func voidCommand(method, url string, params interface{}) error {
	if params == nil {
		params = make(map[string]interface{})
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = executeCommand(method, url, data)
	return err
}

func (wd *WebDriver) voidCommand(urlTemplate string, params interface{}) error {
	return voidCommand("POST", wd.requestURL(urlTemplate, wd.id), params)
}

func (wd *WebDriver) stringsCommand(urlTemplate string) ([]string, error) {
	url := wd.requestURL(urlTemplate, wd.id)
	response, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}

	reply := new(struct{ Value []string })
	if err := json.Unmarshal(response, reply); err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func (wd *WebDriver) boolCommand(urlTemplate string) (bool, error) {
	url := wd.requestURL(urlTemplate, wd.id)
	response, err := wd.execute("GET", url, nil)
	if err != nil {
		return false, err
	}

	reply := new(struct{ Value bool })
	if err := json.Unmarshal(response, reply); err != nil {
		return false, err
	}

	return reply.Value, nil
}

func (wd *WebDriver) Status() (*Status, error) {
	url := wd.requestURL("/status")
	reply, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}

	status := new(struct{ Value Status })
	if err := json.Unmarshal(reply, status); err != nil {
		return nil, err
	}

	return &status.Value, nil
}

// parseVersion sanitizes the browser version enough for semver.ParseTolerant
// to parse it.
func parseVersion(v string) (semver.Version, error) {
	parts := strings.Split(v, ".")
	var err error
	for i := len(parts); i > 0; i-- {
		var ver semver.Version
		ver, err = semver.ParseTolerant(strings.Join(parts[:i], "."))
		if err == nil {
			return ver, nil
		}
	}
	return semver.Version{}, err
}

// The list of valid, top-level capability names, according to the W3C
// specification.
//
// This must be kept in sync with the specification:
// https://www.w3.org/TR/webdriver/#capabilities
var w3cCapabilityNames = []string{
	"acceptInsecureCerts",
	"browserName",
	"browserVersion",
	"platformName",
	"pageLoadStrategy",
	"proxy",
	"setWindowRect",
	"timeouts",
	"unhandledPromptBehavior",
}

var chromeCapabilityNames = []string{
	// This is not a standardized top-level capability name, but Chromedriver
	// expects this capability here.
	// https://cs.chromium.org/chromium/src/chrome/test/chromedriver/capabilities.cc?rcl=0754b5d0aad903439a628618f0e41845f1988f0c&l=759
	"loggingPrefs",
}

// Create a W3C-compatible capabilities instance.
func newW3CCapabilities(caps Capabilities) Capabilities {
	isValidW3CCapability := map[string]bool{}
	for _, name := range w3cCapabilityNames {
		isValidW3CCapability[name] = true
	}
	if b, ok := caps["browserName"]; ok && b == "chrome" {
		for _, name := range chromeCapabilityNames {
			isValidW3CCapability[name] = true
		}
	}

	alwaysMatch := make(Capabilities)
	for name, value := range caps {
		if isValidW3CCapability[name] || strings.Contains(name, ":") {
			alwaysMatch[name] = value
		}
	}

	// Move the Firefox profile setting from the old location to the new
	// location.
	if prof, ok := caps["firefox_profile"]; ok {
		if c, ok := alwaysMatch[firefox.CapabilitiesKey]; ok {
			firefoxCaps := c.(firefox.Capabilities)
			if firefoxCaps.Profile == "" {
				firefoxCaps.Profile = prof.(string)
			}
		} else {
			alwaysMatch[firefox.CapabilitiesKey] = firefox.Capabilities{
				Profile: prof.(string),
			}
		}
	}

	return Capabilities{
		"alwaysMatch": alwaysMatch,
	}
}

func (wd *WebDriver) NewSession() (string, error) {
	// Detect whether the remote end complies with the W3C specification:
	// non-compliant implementations use the top-level 'desiredCapabilities' JSON
	// key, whereas the specification mandates the 'capabilities' key.
	//
	// However, Selenium 3 currently does not implement this part of the specification.
	// https://github.com/SeleniumHQ/selenium/issues/2827
	//
	// TODO(minusnine): audit which ones of these are still relevant. The W3C
	// standard switched to the "alwaysMatch" version in February 2017.
	attempts := []struct {
		params map[string]interface{}
	}{
		{map[string]interface{}{
			"capabilities":        newW3CCapabilities(wd.capabilities),
			"desiredCapabilities": wd.capabilities,
		}},
		{map[string]interface{}{
			"capabilities": map[string]interface{}{
				"desiredCapabilities": wd.capabilities,
			},
		}},
		{map[string]interface{}{
			"desiredCapabilities": wd.capabilities,
		}}}

	for i, s := range attempts {
		data, err := json.Marshal(s.params)
		if err != nil {
			return "", err
		}

		response, err := wd.execute("POST", wd.requestURL("/session"), data)
		if err != nil {
			return "", err
		}

		reply := new(serverReply)
		if err := json.Unmarshal(response, reply); err != nil {
			if i < len(attempts) {
				continue
			}
			return "", err
		}
		if reply.Status != 0 && i < len(attempts) {
			continue
		}
		if reply.SessionID != nil {
			wd.id = *reply.SessionID
		}

		if len(reply.Value) > 0 {
			type returnedCapabilities struct {
				// firefox via geckodriver: 55.0a1
				BrowserVersion string
				// chrome via chromedriver: 61.0.3116.0
				// firefox via selenium 2: 45.9.0
				// htmlunit: 9.4.3.v20170317
				Version          string
				PageLoadStrategy string
				Proxy            Proxy
				Timeouts         struct {
					Implicit       float32
					PageLoadLegacy float32 `json:"page load"`
					PageLoad       float32
					Script         float32
				}
			}

			value := struct {
				SessionID string

				// The W3C specification moved most of the returned data into the
				// "capabilities" field.
				Capabilities *returnedCapabilities

				// Legacy implementations returned most data directly in the "values"
				// key.
				returnedCapabilities
			}{}

			if err := json.Unmarshal(reply.Value, &value); err != nil {
				return "", fmt.Errorf("error unmarshalling value: %v", err)
			}
			if value.SessionID != "" && wd.id == "" {
				wd.id = value.SessionID
			}
			var caps returnedCapabilities
			if value.Capabilities != nil {
				caps = *value.Capabilities
				wd.w3cCompatible = true
			} else {
				caps = value.returnedCapabilities
			}

			for _, s := range []string{caps.Version, caps.BrowserVersion} {
				if s == "" {
					continue
				}
				v, err := parseVersion(s)
				if err != nil {
					logs.Error("error parsing version: %v\n", err)
					continue
				}
				wd.browserVersion = v
			}
		}

		return wd.id, nil
	}
	panic("unreachable")
}

func (wd *WebDriver) Capabilities() (Capabilities, error) {
	url := wd.requestURL("/session/%s", wd.id)
	response, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c := new(struct{ Value Capabilities })
	if err := json.Unmarshal(response, c); err != nil {
		return nil, err
	}

	return c.Value, nil
}

func (wd *WebDriver) SetAsyncScriptTimeout(timeout time.Duration) error {
	if !wd.w3cCompatible {
		return wd.voidCommand("/session/%s/timeouts/async_script", map[string]uint{
			"ms": uint(timeout / time.Millisecond),
		})
	}
	return wd.voidCommand("/session/%s/timeouts", map[string]uint{
		"script": uint(timeout / time.Millisecond),
	})
}

func (wd *WebDriver) SetImplicitWaitTimeout(timeout time.Duration) error {
	if !wd.w3cCompatible {
		return wd.voidCommand("/session/%s/timeouts/implicit_wait", map[string]uint{
			"ms": uint(timeout / time.Millisecond),
		})
	}
	return wd.voidCommand("/session/%s/timeouts", map[string]uint{
		"implicit": uint(timeout / time.Millisecond),
	})
}

func (wd *WebDriver) SetPageLoadTimeout(timeout time.Duration) error {
	body := g.Map{}
	if !wd.w3cCompatible {
		body["ms"] = uint(timeout / time.Millisecond)
		body["type"] = "page load"
	} else {
		body["pageLoad"] = uint(timeout / time.Millisecond)
	}
	_, err := wd.request(setTimeout, "", "", "", body)
	return err
}

func (wd *WebDriver) Quit() error {
	if wd.id == "" {
		return nil
	}
	api := getApi(delSession)
	_, err := wd.execute(api.Method, wd.requestURL(api.Path, wd.id), nil)
	if err == nil {
		wd.id = ""
	}
	return err
}

func (wd *WebDriver) CurrentWindowHandle() (string, error) {
	v, err := wd.request(getWindow, "", "", "", nil)
	if err != nil {
		return "", err
	}
	return conv.String(v), nil
}

func (wd *WebDriver) WindowHandles() ([]string, error) {
	v, err := wd.request(getWindows, "", "", "", nil)
	if err != nil {
		return nil, err
	}
	return conv.Strings(v), nil
}

func (wd *WebDriver) CurrentURL() (string, error) {
	v, err := wd.request(getUrl, "", "", "", nil)
	if err != nil {
		return "", err
	}
	return conv.String(v), nil
}

func (wd *WebDriver) Get(url string) error {
	requestURL := wd.requestURL("/session/%s/url", wd.id)
	params := map[string]string{
		"url": url,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = wd.execute("POST", requestURL, data)
	return err
}

func (wd *WebDriver) Forward() error {
	return wd.voidCommand("/session/%s/forward", nil)
}

func (wd *WebDriver) Back() error {
	return wd.voidCommand("/session/%s/back", nil)
}

func (wd *WebDriver) Refresh() error {
	return wd.voidCommand("/session/%s/refresh", nil)
}

func (wd *WebDriver) Title() (string, error) {
	return wd.stringCommand("/session/%s/title")
}

func (wd *WebDriver) PageSource() (string, error) {
	return wd.stringCommand("/session/%s/source")
}

func (wd *WebDriver) Text() (string, error) {
	return wd.PageSource()
}

func (wd *WebDriver) find(by, value, suffix, url string) ([]byte, error) {
	// The W3C specification removed the specific ID and Name locator strategies,
	// instead only providing a CSS-based strategy. Emulate the old behavior to
	// maintain API compatibility.
	if wd.w3cCompatible {
		switch by {
		case ByID:
			by = ByCSSSelector
			value = "#" + value
		case ByName:
			by = ByCSSSelector
			value = fmt.Sprintf("input[name=%q]", value)
		}
	}

	params := map[string]string{
		"using": by,
		"value": value,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	if len(url) == 0 {
		url = "/session/%s/element"
	}

	return wd.execute("POST", wd.requestURL(url+suffix, wd.id), data)
}

func (wd *WebDriver) DecodeElement(data []byte) (*WebElement, error) {
	reply := new(struct{ Value map[string]string })
	if err := json.Unmarshal(data, &reply); err != nil {
		return nil, err
	}

	id := elementIDFromValue(reply.Value)
	if id == "" {
		return nil, fmt.Errorf("invalid element returned: %+v", reply)
	}
	return &WebElement{
		parent: wd,
		id:     id,
	}, nil
}

const (
	// legacyWebElementIdentifier is the string constant used in the old
	// WebDriver JSON protocol that is the key for the map that contains an
	// unique element identifier.
	legacyWebElementIdentifier = "ELEMENT"

	// webElementIdentifier is the string constant defined by the W3C
	// specification that is the key for the map that contains a unique element identifier.
	webElementIdentifier = "element-6066-11e4-a52e-4f735466cecf"
)

func elementIDFromValue(v map[string]string) string {
	for _, key := range []string{webElementIdentifier, legacyWebElementIdentifier} {
		v, ok := v[key]
		if !ok || v == "" {
			continue
		}
		return v
	}
	return ""
}

func (wd *WebDriver) DecodeElements(data []byte) ([]*WebElement, error) {
	reply := new(struct{ Value []map[string]string })
	if err := json.Unmarshal(data, reply); err != nil {
		return nil, err
	}

	elems := make([]*WebElement, len(reply.Value))
	for i, elem := range reply.Value {
		id := elementIDFromValue(elem)
		if id == "" {
			return nil, fmt.Errorf("invalid element returned: %+v", reply)
		}
		elems[i] = &WebElement{
			parent: wd,
			id:     id,
		}
	}

	return elems, nil
}

func (wd *WebDriver) FindElement(by, value string) (*WebElement, error) {
	response, err := wd.find(by, value, "", "")
	if err != nil {
		return nil, err
	}
	return wd.DecodeElement(response)
}

func (wd *WebDriver) FindElements(by, value string) ([]*WebElement, error) {
	response, err := wd.find(by, value, "s", "")
	if err != nil {
		return nil, err
	}

	return wd.DecodeElements(response)
}

// FindAll 查找所有元素
func (this *WebDriver) FindAll(by, value string) ([]*WebElement, error) {
	return this.FindElements(by, value)
}

// Find 查找一个元素
func (this *WebDriver) Find(by, value string) (*WebElement, error) {
	return this.FindElement(by, value)
}

// FindTagAttributes 查找所有标签的属性Attribute,例如a.href
func (this *WebDriver) FindTagAttributes(tag string) ([]string, error) {
	tagList := strings.Split(tag, ".")
	name := tagList[len(tagList)-1]
	es, err := this.FindTags(strings.Join(tagList[:len(tagList)-1], "."))
	if err != nil {
		return nil, err
	}
	list := []string(nil)
	for _, v := range es {
		//标签不一定有这个属性,固忽略错误
		s, err := v.GetAttribute(name)
		if err == nil {
			switch s {
			case "", "javascript:;":
			default:
				list = append(list, s)
			}
		}
	}
	return list, nil
}

// RangeTags 遍历标签,例如a标签
func (this *WebDriver) RangeTags(tag string, f func(*WebElement) error) error {
	es, err := this.FindTags(tag)
	if err != nil {
		return err
	}
	for _, e := range es {
		if err = f(e); err != nil {
			return err
		}
	}
	return nil
}

// FindTags 查找所有标签,例如a标签,href在a标签里面
func (this *WebDriver) FindTags(tag string) ([]*WebElement, error) {
	var es []*WebElement
	var err error
	tagList := strings.Split(tag, ".")
	for i, v := range tagList {
		if i == 0 {
			es, err = this.FindElements(ByTagName, v)
			if err != nil {
				return nil, err
			}
		} else {
			es2 := []*WebElement(nil)
			for _, e := range es {
				es, err = e.FindElements(ByTagName, v)
				if err != nil {
					return nil, err
				}
				es2 = append(es2, es...)
			}
			es = es2
		}
	}
	return es, err
}

// FindTag 查找标签,例如a标签
func (this *WebDriver) FindTag(tag string) (*WebElement, error) {
	return this.FindElement(ByTagName, tag)
}

func (wd *WebDriver) Close() error {
	url := wd.requestURL("/session/%s/window", wd.id)
	_, err := wd.execute("DELETE", url, nil)
	return err
}

func (wd *WebDriver) SwitchWindow(name string) error {
	params := make(map[string]string)
	if !wd.w3cCompatible {
		params["name"] = name
	} else {
		params["handle"] = name
	}
	return wd.voidCommand("/session/%s/window", params)
}

func (wd *WebDriver) CloseWindow(name string) error {
	return wd.modifyWindow(name, "DELETE", "", nil)
}

func (wd *WebDriver) MaximizeWindow(name string) error {
	if !wd.w3cCompatible {
		if name != "" {
			var err error
			name, err = wd.CurrentWindowHandle()
			if err != nil {
				return err
			}
		}
		url := wd.requestURL("/session/%s/window/%s/maximize", wd.id, name)
		_, err := wd.execute("POST", url, nil)
		return err
	}
	return wd.modifyWindow(name, "POST", "maximize", map[string]string{})
}

func (wd *WebDriver) MinimizeWindow(name string) error {
	return wd.modifyWindow(name, "POST", "minimize", map[string]string{})
}

func (wd *WebDriver) modifyWindow(name, verb, command string, params interface{}) error {
	// The original protocol allowed for maximizing any named window. The W3C
	// specification only allows the current window be be modified. Emulate the
	// previous behavior by switching to the target window, maximizing the
	// current window, and switching back to the original window.
	var startWindow string
	if name != "" && wd.w3cCompatible {
		var err error
		startWindow, err = wd.CurrentWindowHandle()
		if err != nil {
			return err
		}
		if name != startWindow {
			if err := wd.SwitchWindow(name); err != nil {
				return err
			}
		}
	}

	url := wd.requestURL("/session/%s/window", wd.id)
	if command != "" {
		if wd.w3cCompatible {
			url = wd.requestURL("/session/%s/window/%s", wd.id, command)
		} else {
			url = wd.requestURL("/session/%s/window/%s/%s", wd.id, name, command)
		}
	}

	var data []byte
	if params != nil {
		var err error
		if data, err = json.Marshal(params); err != nil {
			return err
		}
	}

	if _, err := wd.execute(verb, url, data); err != nil {
		return err
	}

	// TODO(minusnine): add a test for switching back to the original window.
	if name != startWindow && wd.w3cCompatible {
		if err := wd.SwitchWindow(startWindow); err != nil {
			return err
		}
	}

	return nil
}

func (wd *WebDriver) ResizeWindow(name string, width, height int) error {
	if !wd.w3cCompatible {
		return wd.modifyWindow(name, "POST", "size", map[string]int{
			"width":  width,
			"height": height,
		})
	}
	return wd.modifyWindow(name, "POST", "rect", map[string]float64{
		"width":  float64(width),
		"height": float64(height),
	})
}

func (wd *WebDriver) SwitchFrame(frame interface{}) error {
	params := map[string]interface{}{}
	switch f := frame.(type) {
	case WebElement, int, nil:
		params["id"] = f
	case string:
		if f == "" {
			params["id"] = nil
		} else if wd.w3cCompatible {
			e, err := wd.FindElement(ByID, f)
			if err != nil {
				return err
			}
			params["id"] = e
		} else { // Legacy, non W3C-spec behavior.
			params["id"] = f
		}
	default:
		return fmt.Errorf("invalid type %T", frame)
	}
	return wd.voidCommand("/session/%s/frame", params)
}

func (wd *WebDriver) ActiveElement() (*WebElement, error) {
	verb := "GET"
	if wd.browser == "firefox" && wd.browserVersion.Major < 47 {
		verb = "POST"
	}
	url := wd.requestURL("/session/%s/element/active", wd.id)
	response, err := wd.execute(verb, url, nil)
	if err != nil {
		return nil, err
	}
	return wd.DecodeElement(response)
}

// ChromeDriver returns the expiration date as a float. Handle both formats
// via a type switch.
type cookie struct {
	Name     string      `json:"name"`
	Value    string      `json:"value"`
	Path     string      `json:"path"`
	Domain   string      `json:"domain"`
	Secure   bool        `json:"secure"`
	Expiry   interface{} `json:"expiry"`
	HTTPOnly bool        `json:"httpOnly"`
	SameSite string      `json:"sameSite",omitempty`
}

func (c cookie) sanitize() Cookie {
	parseExpiry := func(e interface{}) uint {
		switch expiry := c.Expiry.(type) {
		case int:
			if expiry > 0 {
				return uint(expiry)
			}
		case float64:
			return uint(expiry)
		}
		return 0
	}

	parseSameSite := func(s string) SameSite {
		if s == "" {
			return ""
		}
		for _, v := range []SameSite{SameSiteNone, SameSiteLax, SameSiteStrict} {
			if strings.EqualFold(string(v), s) {
				return v
			}
		}
		return SameSiteLax
	}

	return Cookie{
		Name:     c.Name,
		Value:    c.Value,
		Path:     c.Path,
		Domain:   c.Domain,
		Secure:   c.Secure,
		Expiry:   parseExpiry(c.Expiry),
		HTTPOnly: c.HTTPOnly,
		SameSite: parseSameSite(c.SameSite),
	}
}

func (wd *WebDriver) GetCookie(name string) (Cookie, error) {
	if wd.browser == "chrome" {
		cs, err := wd.GetCookies()
		if err != nil {
			return Cookie{}, err
		}
		for _, c := range cs {
			if c.Name == name {
				return c, nil
			}
		}
		return Cookie{}, errors.New("cookie not found")
	}
	url := wd.requestURL("/session/%s/cookie/%s", wd.id, name)
	data, err := wd.execute("GET", url, nil)
	if err != nil {
		return Cookie{}, err
	}

	// GeckoDriver returns a list of cookies for this method. Try both a single
	// cookie and a list.
	//
	// https://github.com/mozilla/geckodriver/issues/761
	reply := new(struct{ Value cookie })
	if err := json.Unmarshal(data, reply); err == nil {
		return reply.Value.sanitize(), nil
	}
	listReply := new(struct{ Value []cookie })
	if err := json.Unmarshal(data, listReply); err != nil {
		return Cookie{}, err
	}
	if len(listReply.Value) == 0 {
		return Cookie{}, errors.New("no cookies returned")
	}
	return listReply.Value[0].sanitize(), nil
}

func (wd *WebDriver) GetCookies() ([]Cookie, error) {
	v, err := wd.request(getCookies, "", "", "", nil)
	if err != nil {
		return nil, err
	}
	reply := []cookie(nil)
	if err := json.Unmarshal(conv.Bytes(v), &reply); err != nil {
		return nil, err
	}

	cookies := make([]Cookie, len(reply))
	for i, c := range reply {
		cookies[i] = c.sanitize()
	}
	return cookies, nil
}

func (wd *WebDriver) AddCookie(cookie *Cookie) error {
	return wd.voidCommand("/session/%s/cookie", map[string]*Cookie{
		"cookie": cookie,
	})
}

func (wd *WebDriver) DeleteAllCookies() error {
	url := wd.requestURL("/session/%s/cookie", wd.id)
	_, err := wd.execute("DELETE", url, nil)
	return err
}

func (wd *WebDriver) DeleteCookie(name string) error {
	url := wd.requestURL("/session/%s/cookie/%s", wd.id, name)
	_, err := wd.execute("DELETE", url, nil)
	return err
}

// Click TODO(minusnine): add a test for Click.
func (wd *WebDriver) Click(button int) error {
	return wd.voidCommand("/session/%s/click", map[string]int{
		"button": button,
	})
}

// DoubleClick TODO(minusnine): add a test for DoubleClick.
func (wd *WebDriver) DoubleClick() error {
	return wd.voidCommand("/session/%s/doubleclick", nil)
}

// ButtonDown TODO(minusnine): add a test for ButtonDown.
func (wd *WebDriver) ButtonDown() error {
	return wd.voidCommand("/session/%s/buttondown", nil)
}

// ButtonUp TODO(minusnine): add a test for ButtonUp.
func (wd *WebDriver) ButtonUp() error {
	return wd.voidCommand("/session/%s/buttonup", nil)
}

func (wd *WebDriver) SendModifier(modifier string, isDown bool) error {
	if isDown {
		return wd.KeyDown(modifier)
	}
	return wd.KeyUp(modifier)
}

func (wd *WebDriver) keyAction(action, keys string) error {
	type keyAction struct {
		Type string `json:"type"`
		Key  string `json:"value"`
	}
	actions := make([]keyAction, 0, len(keys))
	for _, key := range keys {
		actions = append(actions, keyAction{
			Type: action,
			Key:  string(key),
		})
	}
	return wd.voidCommand("/session/%s/actions", map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{
				"type":    "key",
				"id":      "default keyboard",
				"actions": actions,
			}},
	})
}

func (wd *WebDriver) KeyDown(keys string) error {
	// Selenium implemented the actions API but has not yet updated its new
	// session response.
	if !wd.w3cCompatible && !(wd.browser == "firefox" && wd.browserVersion.Major > 47) {
		return wd.voidCommand("/session/%s/keys", wd.processKeyString(keys))
	}
	return wd.keyAction("keyDown", keys)
}

func (wd *WebDriver) KeyUp(keys string) error {
	// Selenium implemented the actions API but has not yet updated its new
	// session response.
	if !wd.w3cCompatible && !(wd.browser == "firefox" && wd.browserVersion.Major > 47) {
		return wd.KeyDown(keys)
	}
	return wd.keyAction("keyUp", keys)
}

// KeyPauseAction builds a KeyAction which pauses for the supplied duration.
func KeyPauseAction(duration time.Duration) KeyAction {
	return KeyAction{
		"type":     "pause",
		"duration": uint(duration / time.Millisecond),
	}
}

// KeyUpAction builds a KeyAction press.
func KeyUpAction(key string) KeyAction {
	return KeyAction{
		"type":  "keyUp",
		"value": key,
	}
}

// KeyDownAction builds a KeyAction which presses and holds
// the specified key.
func KeyDownAction(key string) KeyAction {
	return KeyAction{
		"type":  "keyDown",
		"value": key,
	}
}

// PointerPauseAction PointerPause builds a PointerAction which pauses for the supplied duration.
func PointerPauseAction(duration time.Duration) PointerAction {
	return PointerAction{
		"type":     "pause",
		"duration": uint(duration / time.Millisecond),
	}
}

// PointerMoveAction PointerMove builds a PointerAction which moves the pointer.
func PointerMoveAction(duration time.Duration, offset Point, origin PointerMoveOrigin) PointerAction {
	return PointerAction{
		"type":     "pointerMove",
		"duration": uint(duration / time.Millisecond),
		"origin":   origin,
		"x":        offset.X,
		"y":        offset.Y,
	}
}

// PointerUpAction PointerUp builds an action which releases the specified pointer key.
func PointerUpAction(button MouseButton) PointerAction {
	return PointerAction{
		"type":   "pointerUp",
		"button": button,
	}
}

// PointerDownAction PointerDown builds a PointerAction which presses
// and holds the specified pointer key.
func PointerDownAction(button MouseButton) PointerAction {
	return PointerAction{
		"type":   "pointerDown",
		"button": button,
	}
}

func (wd *WebDriver) StoreKeyActions(inputID string, actions ...KeyAction) {
	rawActions := []map[string]interface{}{}
	for _, action := range actions {
		rawActions = append(rawActions, action)
	}
	wd.storedActions = append(wd.storedActions, map[string]interface{}{
		"type":    "key",
		"id":      inputID,
		"actions": rawActions,
	})
}

func (wd *WebDriver) StorePointerActions(inputID string, pointer PointerType, actions ...PointerAction) {
	rawActions := []map[string]interface{}{}
	for _, action := range actions {
		rawActions = append(rawActions, action)
	}
	wd.storedActions = append(wd.storedActions, map[string]interface{}{
		"type":       "pointer",
		"id":         inputID,
		"parameters": map[string]string{"pointerType": string(pointer)},
		"actions":    rawActions,
	})
}

func (wd *WebDriver) PerformActions() error {
	err := wd.voidCommand("/session/%s/actions", map[string]interface{}{
		"actions": wd.storedActions,
	})
	wd.storedActions = nil
	return err
}

func (wd *WebDriver) ReleaseActions() error {
	return voidCommand("DELETE", wd.requestURL("/session/%s/actions", wd.id), nil)
}

func (wd *WebDriver) DismissAlert() error {
	return wd.voidCommand("/session/%s/alert/dismiss", nil)
}

func (wd *WebDriver) AcceptAlert() error {
	return wd.voidCommand("/session/%s/alert/accept", nil)
}

func (wd *WebDriver) AlertText() (string, error) {
	return wd.stringCommand("/session/%s/alert/text")
}

func (wd *WebDriver) SetAlertText(text string) error {
	data := map[string]string{"text": text}
	return wd.voidCommand("/session/%s/alert/text", data)
}

func (wd *WebDriver) execScriptRaw(script string, args []interface{}, suffix string) ([]byte, error) {
	if args == nil {
		args = make([]interface{}, 0)
	}

	data, err := json.Marshal(map[string]interface{}{
		"script": script,
		"args":   args,
	})
	if err != nil {
		return nil, err
	}

	return wd.execute("POST", wd.requestURL("/session/%s/execute"+suffix, wd.id), data)
}

func (wd *WebDriver) execScript(script string, args []interface{}, suffix string) (interface{}, error) {
	response, err := wd.execScriptRaw(script, args, suffix)
	if err != nil {
		return nil, err
	}

	reply := new(struct{ Value interface{} })
	if err = json.Unmarshal(response, reply); err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func (wd *WebDriver) ExecuteScript(script string, args []interface{}) (interface{}, error) {
	if !wd.w3cCompatible {
		return wd.execScript(script, args, "")
	}
	return wd.execScript(script, args, "/sync")
}

func (wd *WebDriver) ExecuteScriptAsync(script string, args []interface{}) (interface{}, error) {
	if !wd.w3cCompatible {
		return wd.execScript(script, args, "_async")
	}
	return wd.execScript(script, args, "/async")
}

func (wd *WebDriver) ExecuteScriptRaw(script string, args []interface{}) ([]byte, error) {
	if !wd.w3cCompatible {
		return wd.execScriptRaw(script, args, "")
	}
	return wd.execScriptRaw(script, args, "/sync")
}

func (wd *WebDriver) ExecuteScriptAsyncRaw(script string, args []interface{}) ([]byte, error) {
	if !wd.w3cCompatible {
		return wd.execScriptRaw(script, args, "_async")
	}
	return wd.execScriptRaw(script, args, "/async")
}

// Screenshot 截图信息
func (wd *WebDriver) Screenshot() ([]byte, error) {
	v, err := wd.request(screenshot, "", "", "", nil)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(conv.String(v))
}

// SaveScreenshot 保存截图
func (wd *WebDriver) SaveScreenshot(filename string) error {
	data, err := wd.Screenshot()
	if err != nil {
		return err
	}
	return oss.New(filename, data)
}

// Condition is an alias for a type that is passed as an argument
// for selenium.Wait(cond Condition) (error) function.
type Condition func(wd *WebDriver) (bool, error)

func (wd *WebDriver) WaitWithTimeoutAndInterval(condition Condition, timeout, interval time.Duration) error {
	startTime := time.Now()

	for {
		done, err := condition(wd)
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		if elapsed := time.Since(startTime); elapsed > timeout {
			return fmt.Errorf("timeout after %v", elapsed)
		}
		time.Sleep(interval)
	}
}

func (wd *WebDriver) WaitWithTimeout(condition Condition, timeout time.Duration) error {
	return wd.WaitWithTimeoutAndInterval(condition, timeout, DefaultWaitInterval)
}

func (wd *WebDriver) Wait(condition Condition) error {
	return wd.WaitWithTimeoutAndInterval(condition, DefaultWaitTimeout, DefaultWaitInterval)
}

func (wd *WebDriver) Log(typ log.Type) ([]log.Message, error) {
	url := wd.requestURL("/session/%s/log", wd.id)
	params := map[string]log.Type{
		"type": typ,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	response, err := wd.execute("POST", url, data)
	if err != nil {
		return nil, err
	}

	c := new(struct {
		Value []struct {
			Timestamp int64
			Level     string
			Message   string
		}
	})
	if err = json.Unmarshal(response, c); err != nil {
		return nil, err
	}

	val := make([]log.Message, len(c.Value))
	for i, v := range c.Value {
		val[i] = log.Message{
			// n.b.: Chrome, which is the only browser that supports this API,
			// supplies timestamps in milliseconds since the Epoch.
			Timestamp: time.Unix(0, v.Timestamp*int64(time.Millisecond)),
			Level:     log.Level(v.Level),
			Message:   v.Message,
		}
	}

	return val, nil
}

func (wd *WebDriver) request(key, elementID, shadowID, name string, body interface{}) (interface{}, error) {
	api := getApi2(key, wd.id, elementID, shadowID, name)
	req, err := http.NewRequest(api.Method, wd.urlPrefix+api.Path, bytes.NewReader(conv.Bytes(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", jsonContentType)
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码错误: %s", resp.Status)
	}
	m := conv.NewMap(resp.Body)
	status := m.GetInt("status", -1)
	if status == -1 {
		return nil, errors.New("未知错误: " + m.String())
	}
	if status != 0 {
		return nil, errors.New(remoteErrors[status])
	}
	return m.GetInterface("value"), nil
}
