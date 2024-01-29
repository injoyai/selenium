package selenium

import (
	"github.com/injoyai/conv"
	"github.com/injoyai/goutil/g"
	"github.com/injoyai/goutil/net/http"
	"github.com/injoyai/goutil/oss"
	"github.com/injoyai/selenium/chrome"
	"path/filepath"
	"strings"
)

func Chrome(driverPath, browserPath string, option ...Option) (*WebDriver, error) {
	e, err := New(driverPath, browserPath, option...)
	if err != nil {
		return nil, err
	}
	return e.WebDriver()
}

func New(driverPath, browserPath string, option ...Option) (*Entity, error) {
	browserName := strings.Split(filepath.Base(browserPath), ".")[0]
	if browserName != _chrome && browserName != _firefox {
		browserName = _chrome
	}
	e := &Entity{
		Service:     &Service{},
		browserName: browserName,
		browserPath: browserPath,
	}
	if err := e.SetUrl(DefaultURLPrefix); err != nil {
		return nil, err
	}
	for _, v := range option {
		if err := v(e); err != nil {
			return nil, err
		}
	}
	err := e.Service.startChrome(driverPath)
	return e, err
}

const (
	_chrome  = "chrome"
	_firefox = "firefox"
	_opera   = "opera"
)

type Option func(e *Entity) error

type Entity struct {
	*Service

	browserName string //浏览器名称
	browserPath string //浏览器目录
	Prefs       map[string]interface{}
	Args        []string
}

func (this *Entity) SetPref(key string, value interface{}) *Entity {
	this.Prefs[key] = value
	return this
}

func (this *Entity) AddArgument(args ...string) *Entity {
	this.Args = append(this.Args, args...)
	return this
}

func (this *Entity) DelArgument(arg string) *Entity {
	for i, v := range this.Args {
		if v == arg {
			this.Args = append(this.Args[:i], this.Args[i+1:]...)
			break
		}
	}
	return this
}

func (this *Entity) SetProxy(u string) *Entity {
	return this.AddArgument("--proxy-server=" + u)
}

// ShowWindow 显示窗口linux系统无效
func (this *Entity) ShowWindow(b ...bool) *Entity {
	if !oss.IsWindows() || (len(b) > 0 && !b[0]) {
		this.AddArgument("--headless")
	} else {
		this.DelArgument("--headless")
	}
	return this
}

// ShowImg 是否加载图片
func (this *Entity) ShowImg(b ...bool) *Entity {
	show := oss.IsWindows() && !(len(b) > 0 && !b[0])
	arg := conv.SelectInt(show, 1, 2)
	return this.SetPref("profile.managed_default_content_settings.images", arg)
}

// SetBrowser 设置浏览器,目前只测试了chrome
func (this *Entity) SetBrowser(b string) *Entity {
	this.browserName = b
	return this
}

// SetBrowserPath 设置浏览器目录
func (this *Entity) SetBrowserPath(p string) *Entity {
	this.browserPath = p
	return this
}

// SetUserAgent 设置UserAgent
func (this *Entity) SetUserAgent(ua string) *Entity {
	return this.AddArgument("--user-agent=" + ua)
}

// SetUserAgentDefault 设置UserAgent到默认值
func (this *Entity) SetUserAgentDefault() *Entity {
	return this.SetUserAgent(http.UserAgentDefault)
}

// SetUserAgentRand 设置随机UserAgent
func (this *Entity) SetUserAgentRand() *Entity {
	idx := g.RandInt(0, len(http.UserAgentList)-1)
	return this.SetUserAgent(http.UserAgentList[idx])
}

func (this *Entity) Close() {
	this.Service.Stop()
}

func (this *Entity) WebDriver() (*WebDriver, error) {
	//链接本地的浏览器 chrome
	caps := Capabilities{"browserName": this.browserName}
	caps.AddChrome(chrome.Capabilities{
		Path:  this.browserPath,
		Prefs: this.Prefs,
		Args:  this.Args,
	})
	//调起浏览器
	return NewRemote(caps, this.Service.GetUrl())
}

// Run 执行
func (this *Entity) Run(f func(w *WebDriver) error, retry ...uint) error {
	// 调起浏览器
	web, err := this.WebDriver()
	if err != nil {
		return err
	}
	defer web.Close()
	return g.Retry(func() error { return f(web) }, retry...)
}
