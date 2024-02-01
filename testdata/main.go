package main

import (
	"github.com/injoyai/goutil/oss"
	"github.com/injoyai/logs"
	"github.com/injoyai/selenium"
	"time"
)

func main() {

	selenium.Debug(true)
	//logs.DefaultWrite.SetColor(color.FgYellow)
	//logs.DefaultRead.SetColor(color.FgYellow)
	wb, err := selenium.Chrome(
		oss.UserInjoyDir("/browser/chrome/chromedriver.exe"),
		oss.UserInjoyDir("/browser/chrome/chrome.exe"),
		func(e *selenium.Entity) error {
			e.SetProxy("127.0.0.1:1081")
			return nil
		})
	if err != nil {
		logs.Error(err)
		return
	}
	wb.Get("https://www.baidu.com")

	logs.Debug(wb.SaveScreenshot("./testdata/screenshot.png"))
	for {
		<-time.After(time.Second * 5)
		logs.Debug(wb.WindowHandles())
		logs.Debug(wb.CurrentWindowHandle())
		logs.Debug(wb.CurrentURL())
		logs.Debug(wb.GetCookies())

	}

}
