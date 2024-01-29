package selenium

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/injoyai/goutil/oss"
	"io"
)

type WebElement struct {
	wait
	parent *WebDriver
	// Prior to the W3C specification, elements would be returned as a map with
	// the literal key "ELEMENT" and a value of a UUID. The W3C specification
	// specifies that this key has changed to an UUID-based string constant and
	// that the value is called a "reference". For ease of transition, we store
	// the "reference" in this now misnamed field.
	id string
}

func (elem *WebElement) Click() error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/click", elem.id)
	return elem.parent.voidCommand(urlTemplate, nil)
}

func (elem *WebElement) SendKeys(keys string) error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/value", elem.id)
	return elem.parent.voidCommand(urlTemplate, elem.parent.processKeyString(keys))
}

func (wd *WebDriver) processKeyString(keys string) interface{} {
	if !wd.w3cCompatible {
		chars := make([]string, len(keys))
		for i, c := range keys {
			chars[i] = string(c)
		}
		return map[string][]string{"value": chars}
	}
	return map[string]string{"text": keys}
}

func (elem *WebElement) TagName() (string, error) {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/name", elem.id)
	return elem.parent.stringCommand(urlTemplate)
}

func (elem *WebElement) Text() (string, error) {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/text", elem.id)
	return elem.parent.stringCommand(urlTemplate)
}

func (elem *WebElement) Submit() error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/submit", elem.id)
	return elem.parent.voidCommand(urlTemplate, nil)
}

func (elem *WebElement) Clear() error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/clear", elem.id)
	return elem.parent.voidCommand(urlTemplate, nil)
}

func (elem *WebElement) MoveTo(xOffset, yOffset int) error {
	return elem.parent.voidCommand("/session/%s/moveto", map[string]interface{}{
		"element": elem.id,
		"xoffset": xOffset,
		"yoffset": yOffset,
	})
}

func (elem *WebElement) FindElement(by, value string) (*WebElement, error) {
	url := fmt.Sprintf("/session/%%s/element/%s/element", elem.id)
	response, err := elem.parent.find(by, value, "", url)
	if err != nil {
		return nil, err
	}

	return elem.parent.DecodeElement(response)
}

func (elem *WebElement) FindElements(by, value string) ([]*WebElement, error) {
	url := fmt.Sprintf("/session/%%s/element/%s/element", elem.id)
	response, err := elem.parent.find(by, value, "s", url)
	if err != nil {
		return nil, err
	}

	return elem.parent.DecodeElements(response)
}

func (elem *WebElement) boolQuery(urlTemplate string) (bool, error) {
	return elem.parent.boolCommand(fmt.Sprintf(urlTemplate, elem.id))
}

func (elem *WebElement) IsSelected() (bool, error) {
	return elem.boolQuery("/session/%%s/element/%s/selected")
}

func (elem *WebElement) IsEnabled() (bool, error) {
	return elem.boolQuery("/session/%%s/element/%s/enabled")
}

func (elem *WebElement) IsDisplayed() (bool, error) {
	return elem.boolQuery("/session/%%s/element/%s/displayed")
}

func (elem *WebElement) GetProperty(name string) (string, error) {
	template := "/session/%%s/element/%s/property/%s"
	urlTemplate := fmt.Sprintf(template, elem.id, name)

	return elem.parent.stringCommand(urlTemplate)
}

func (elem *WebElement) GetAttribute(name string) (string, error) {
	template := "/session/%%s/element/%s/attribute/%s"
	urlTemplate := fmt.Sprintf(template, elem.id, name)

	return elem.parent.stringCommand(urlTemplate)
}

func round(f float64) int {
	if f < -0.5 {
		return int(f - 0.5)
	}
	if f > 0.5 {
		return int(f + 0.5)
	}
	return 0
}

func (elem *WebElement) location(suffix string) (*Point, error) {
	if !elem.parent.w3cCompatible {
		wd := elem.parent
		path := "/session/%s/element/%s/location" + suffix
		url := wd.requestURL(path, wd.id, elem.id)
		response, err := wd.execute("GET", url, nil)
		if err != nil {
			return nil, err
		}
		reply := new(struct{ Value rect })
		if err := json.Unmarshal(response, reply); err != nil {
			return nil, err
		}
		return &Point{round(reply.Value.X), round(reply.Value.Y)}, nil
	}

	rect, err := elem.rect()
	if err != nil {
		return nil, err
	}
	return &Point{round(rect.X), round(rect.Y)}, nil
}

func (elem *WebElement) Location() (*Point, error) {
	return elem.location("")
}

func (elem *WebElement) LocationInView() (*Point, error) {
	return elem.location("_in_view")
}

func (elem *WebElement) Size() (*Size, error) {
	if !elem.parent.w3cCompatible {
		wd := elem.parent
		url := wd.requestURL("/session/%s/element/%s/size", wd.id, elem.id)
		response, err := wd.execute("GET", url, nil)
		if err != nil {
			return nil, err
		}
		reply := new(struct{ Value rect })
		if err := json.Unmarshal(response, reply); err != nil {
			return nil, err
		}
		return &Size{round(reply.Value.Width), round(reply.Value.Height)}, nil
	}

	rect, err := elem.rect()
	if err != nil {
		return nil, err
	}

	return &Size{round(rect.Width), round(rect.Height)}, nil
}

type rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// rect implements the "Get Element Rect" method of the W3C standard.
func (elem *WebElement) rect() (*rect, error) {
	wd := elem.parent
	url := wd.requestURL("/session/%s/element/%s/rect", wd.id, elem.id)
	response, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}
	r := new(struct{ Value rect })
	if err := json.Unmarshal(response, r); err != nil {
		return nil, err
	}
	return &r.Value, nil
}

func (elem *WebElement) CSSProperty(name string) (string, error) {
	wd := elem.parent
	return wd.stringCommand(fmt.Sprintf("/session/%%s/element/%s/css/%s", elem.id, name))
}

func (elem *WebElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"ELEMENT":            elem.id,
		webElementIdentifier: elem.id,
	})
}

func (elem *WebElement) Screenshot() ([]byte, error) {
	data, err := elem.parent.stringCommand(fmt.Sprintf("/session/%%s/element/%s/screenshot", elem.id))
	if err != nil {
		return nil, err
	}

	// Selenium returns a base64 encoded image.
	buf := []byte(data)
	decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(buf))
	return io.ReadAll(decoder)
}

func (elem *WebElement) SaveScreenshot(filename string) error {
	data, err := elem.Screenshot()
	if err != nil {
		return err
	}
	return oss.New(filename, data)
}
