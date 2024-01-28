package main

import (
	"github.com/injoyai/goutil/net/http"
	"github.com/injoyai/goutil/oss"
	"github.com/injoyai/logs"
	"github.com/injoyai/selenium"
	"github.com/injoyai/selenium/chrome"
)

func main() {

	serviceOption := []selenium.ServiceOption{}
	//新建seleniumServer
	service, err := selenium.NewChromeDriverService(
		oss.UserInjoyDir("/browser/chrome/chromedriver.exe"),
		4444,
		serviceOption...,
	)
	if nil != err {
		logs.Error(err)
		return
	}
	defer service.Stop()

	//链接本地的浏览器 chrome
	caps := selenium.Capabilities{"browserName": "chrome"}
	//设置浏览器参数
	caps.AddChrome(chrome.Capabilities{
		Path: oss.UserInjoyDir("/browser/chrome/chrome.exe"),
		Prefs: map[string]interface{}{
			//是否禁止图片加载，加快渲染速度
			"profile.managed_default_content_settings.images": 1,
		},
		Args: []string{"--user-agent=" + http.UserAgentDefault},
	})

	// 调起浏览器
	web, err := selenium.NewRemote(caps, "http://localhost:4444/wd/hub")
	if err != nil {
		logs.Error(err)
		return
	}
	defer web.Close()

	select {}

}
