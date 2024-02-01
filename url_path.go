package selenium

import (
	"net/http"
	"strings"
)

const (
	newSession              = "newSession"
	delSession              = "delSession"
	getStatus               = "getStatus"
	getTimeout              = "getTimeout"
	setTimeout              = "setTimeout"
	openUrl                 = "openUrl"
	getUrl                  = "getUrl"
	back                    = "back"
	forward                 = "forward"
	refresh                 = "refresh"
	getTitle                = "getTitle"
	getWindow               = "getWindowHandle"
	closeWindow             = "closeWindow"
	switchToWindow          = "switchToWindow"
	getWindows              = "getWindowHandles"
	newWindow               = "newWindow"
	switchToFrame           = "switchToFrame"
	switchToParentFrame     = "switchToParentFrame"
	getWindowRect           = "getWindowRect"
	setWindowRect           = "setWindowRect"
	maximizeWindow          = "maximizeWindow"
	minimizeWindow          = "minimizeWindow"
	fullscreenWindow        = "fullscreenWindow"
	getActiveElement        = "getActiveElement"
	getShadowRoot           = "getShadowRoot"
	findElement             = "findElement"
	findElements            = "findElements"
	findElementFromElement  = "findElementFromElement"
	findElementsFromElement = "findElementsFromElement"
	findElementFromShadow   = "findElementFromShadow"
	findElementsFromShadow  = "findElementsFromShadow"
	isElementSelected       = "isElementSelected"
	getElementAttribute     = "getElementAttribute"
	getElementProperty      = "getElementProperty"
	getElementCSSValue      = "getElementCSSValue"
	getElementText          = "getElementText"
	getElementName          = "getElementName"
	getElementRect          = "getElementRect"
	getElementEnabled       = "getElementEnabled"
	getComputedRole         = "getComputedRole"
	getComputedLabel        = "getComputedLabel"
	isElementClicked        = "isElementClicked"
	setElementClear         = "setElementClear"
	setElementValue         = "setElementValue"
	getElementSize          = "getElementSize"
	getSource               = "getSource"
	executeScriptSync       = "executeScriptSync"
	executeScriptAsync      = "executeScriptAsync"
	getCookies              = "getCookies"
	getCookie               = "getCookie"
	addCookie               = "addCookie"
	delCookie               = "delCookie"
	delCookies              = "delCookies"
	addActions              = "addActions"
	delActions              = "delActions"
	dismissAlert            = "dismissAlert"
	acceptAlert             = "acceptAlert"
	getAlertText            = "getAlertText"
	setAlertText            = "setAlertText"
	screenshot              = "screenshot"
	elementScreenshot       = "elementScreenshot"
	print                   = "print"
)

func getApi(key string) api {
	a := apiMap[key]
	a.Path = strings.ReplaceAll(a.Path, "{session id}", "%s")
	a.Path = strings.ReplaceAll(a.Path, "{element id}", "%s")
	a.Path = strings.ReplaceAll(a.Path, "{shadow id}", "%s")
	return a
}

func getApi2(key string, sessionID, elementID, shadowID, name string) api {
	a := apiMap[key]
	a.Path = strings.ReplaceAll(a.Path, "{session id}", sessionID)
	a.Path = strings.ReplaceAll(a.Path, "{element id}", elementID)
	a.Path = strings.ReplaceAll(a.Path, "{shadow id}", shadowID)
	a.Path = strings.ReplaceAll(a.Path, "{name}", shadowID)
	return a
}

var apiMap = map[string]api{
	newSession:              {http.MethodPost, "/session"},
	delSession:              {http.MethodDelete, "/session/{session id}"},
	getStatus:               {http.MethodGet, "/status"},
	getTimeout:              {http.MethodGet, "/session/{session id}/timeouts"},
	setTimeout:              {http.MethodPost, "/session/{session id}/timeouts"},
	openUrl:                 {http.MethodPost, "/session/{session id}/url"},
	getUrl:                  {http.MethodGet, "/session/{session id}/url"},
	back:                    {http.MethodPost, "/session/{session id}/back"},
	forward:                 {http.MethodPost, "/session/{session id}/forward"},
	refresh:                 {http.MethodPost, "/session/{session id}/refresh"},
	getTitle:                {http.MethodGet, "/session/{session id}/title"},
	getWindow:               {http.MethodGet, "/session/{session id}/window"},
	closeWindow:             {http.MethodDelete, "/session/{session id}/window"},
	switchToWindow:          {http.MethodPost, "/session/{session id}/window"},
	getWindows:              {http.MethodGet, "/session/{session id}/window/handles"},
	newWindow:               {http.MethodPost, "/session/{session id}/window/new"},
	switchToFrame:           {http.MethodPost, "/session/{session id}/frame"},
	switchToParentFrame:     {http.MethodPost, "/session/{session id}/frame/parent"},
	getWindowRect:           {http.MethodGet, "/session/{session id}/window/rect"},
	setWindowRect:           {http.MethodPost, "/session/{session id}/window/rect"},
	maximizeWindow:          {http.MethodPost, "/session/{session id}/window/maximize"},
	minimizeWindow:          {http.MethodPost, "/session/{session id}/window/minimize"},
	fullscreenWindow:        {http.MethodPost, "/session/{session id}/window/fullscreen"},
	getActiveElement:        {http.MethodGet, "/session/{session id}/element/active"},
	getShadowRoot:           {http.MethodGet, "/session/{session id}/element/{element id}/shadow"},
	findElement:             {http.MethodPost, "/session/{session id}/element"},
	findElements:            {http.MethodPost, "/session/{session id}/elements"},
	findElementFromElement:  {http.MethodPost, "/session/{session id}/element/{element id}/element"},
	findElementsFromElement: {http.MethodPost, "/session/{session id}/element/{element id}/elements"},
	findElementFromShadow:   {http.MethodPost, "/session/{session id}/shadow/{shadow id}/element"},
	findElementsFromShadow:  {http.MethodPost, "/session/{session id}/shadow/{shadow id}/elements"},
	isElementSelected:       {http.MethodGet, "/session/{session id}/element/{element id}/selected"},
	getElementAttribute:     {http.MethodGet, "/session/{session id}/element/{element id}/attribute/{name}"},
	getElementProperty:      {http.MethodGet, "/session/{session id}/element/{element id}/property/{name}"},
	getElementCSSValue:      {http.MethodGet, "/session/{session id}/element/{element id}/css/{property name}"},
	getElementText:          {http.MethodGet, "/session/{session id}/element/{element id}/text"},
	getElementName:          {http.MethodGet, "/session/{session id}/element/{element id}/name"},
	getElementRect:          {http.MethodGet, "/session/{session id}/element/{element id}/rect"},
	getElementEnabled:       {http.MethodGet, "/session/{session id}/element/{element id}/enabled"},
	getComputedRole:         {http.MethodGet, "/session/{session id}/element/{element id}/computedrole"},
	getComputedLabel:        {http.MethodGet, "/session/{session id}/element/{element id}/computedlabel"},
	isElementClicked:        {http.MethodPost, "/session/{session id}/element/{element id}/click"},
	setElementClear:         {http.MethodPost, "/session/{session id}/element/{element id}/clear"},
	setElementValue:         {http.MethodPost, "/session/{session id}/element/{element id}/value"},
	getElementSize:          {http.MethodGet, "/session/{session id}/element/{element id}/size"},
	getSource:               {http.MethodGet, "/session/{session id}/source"},
	executeScriptSync:       {http.MethodPost, "/session/{session id}/execute/sync"},
	executeScriptAsync:      {http.MethodPost, "/session/{session id}/execute/async"},
	getCookies:              {http.MethodGet, "/session/{session id}/cookie"},
	getCookie:               {http.MethodGet, "/session/{session id}/cookie/{name}"},
	addCookie:               {http.MethodPost, "/session/{session id}/cookie"},
	delCookie:               {http.MethodDelete, "/session/{session id}/cookie/{name}"},
	delCookies:              {http.MethodDelete, "/session/{session id}/cookie"},
	addActions:              {http.MethodPost, "/session/{session id}/actions"},
	delActions:              {http.MethodDelete, "/session/{session id}/actions"},
	dismissAlert:            {http.MethodPost, "/session/{session id}/alert/dismiss"},
	acceptAlert:             {http.MethodPost, "/session/{session id}/alert/accept"},
	getAlertText:            {http.MethodGet, "/session/{session id}/alert/text"},
	setAlertText:            {http.MethodPost, "/session/{session id}/alert/text"},
	screenshot:              {http.MethodGet, "/session/{session id}/screenshot"},
	elementScreenshot:       {http.MethodGet, "/session/{session id}/element/{element id}/screenshot"},
	print:                   {http.MethodPost, "/session/{session id}/print"},
}

type api struct {
	Method string
	Path   string
}
