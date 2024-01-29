package selenium

import (
	"github.com/injoyai/conv"
	"time"
)

type wait struct{}

func (wait) Wait(t time.Duration) {
	<-time.After(t)
}

func (this wait) WaitSecond(n ...int) {
	this.Wait(time.Duration(conv.GetDefaultInt(1, n...)) * time.Second)
}

func (this wait) WaitMinute(n ...int) {
	this.Wait(time.Duration(conv.GetDefaultInt(1, n...)) * time.Minute)
}
