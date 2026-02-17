package utils

import (
	"fmt"
	"log"
	"math/rand"
	"mindx/pkg/i18n"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

const chromeDriverPort = 9515

var (
	sharedService *selenium.Service
	serviceMutex  sync.Mutex
)

func isDebug() bool {
	debug := os.Getenv("DEBUG")
	return debug == "true" || debug == "1" || debug == "TRUE"
}

func debugLog(format string, args ...interface{}) {
	if isDebug() {
		log.Printf(format, args...)
	}
}

func debugPrintf(format string, args ...interface{}) {
	if isDebug() {
		fmt.Printf(format, args...)
	}
}

type SearchResult struct {
	Title       string `json:"title"`
	Link        string `json:"url"`
	Description string `json:"summary"`
}

type LinkRef struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type OpenResult struct {
	Content string    `json:"content"`
	Refs    []LinkRef `json:"refs"`
}

type Browser struct {
	wd    selenium.WebDriver
	ownWD bool
}

func StartChromeDriver() error {
	serviceMutex.Lock()
	defer serviceMutex.Unlock()

	if sharedService != nil {
		debugLog("[ChromeDriver] Service already running")
		return nil
	}

	seleniumPath := findChromeDriver()
	if seleniumPath == "" {
		return fmt.Errorf(i18n.T("browser.chrome_driver_not_found"))
	}
	debugLog("[ChromeDriver] Found driver: %s", seleniumPath)

	debugLog("[ChromeDriver] Starting service on port %d...", chromeDriverPort)
	service, err := selenium.NewChromeDriverService(seleniumPath, chromeDriverPort)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("browser.chrome_driver_start_failed", map[string]interface{}{"Error": err.Error()}))
	}

	sharedService = service
	debugLog("[ChromeDriver] Service started successfully on port %d", chromeDriverPort)
	return nil
}

func StopChromeDriver() {
	serviceMutex.Lock()
	defer serviceMutex.Unlock()

	if sharedService != nil {
		sharedService.Stop()
		sharedService = nil
		debugLog("ChromeDriver service stopped")
	}
}

func NewBrowser(proxy string) (*Browser, error) {
	if err := StartChromeDriver(); err != nil {
		return nil, err
	}

	debugLog("[Browser] Creating new browser session...")

	ua := randomUserAgent()
	windowSize := randomWindowSize()
	debugLog("[Browser] User-Agent: %s", ua)
	debugLog("[Browser] Window size: %s", windowSize)
	if proxy != "" {
		debugLog("[Browser] Using proxy: %s", proxy)
	}

	caps := selenium.Capabilities{
		"browserName": "chrome",
		"goog:chromeOptions": map[string]interface{}{
			"excludeSwitches":        []string{"enable-automation"},
			"useAutomationExtension": false,
		},
	}

	args := []string{
		"--headless=new",
		"--user-agent=" + ua,
		"--window-size=" + windowSize,
		"--start-maximized",
		"--disable-blink-features=AutomationControlled",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-gpu",
		"--disable-infobars",
		"--disable-extensions",
	}

	if proxy != "" {
		args = append(args, "--proxy-server="+proxy)
	}

	chromeCaps := chrome.Capabilities{
		Args: args,
	}

	caps.AddChrome(chromeCaps)

	debugLog("[Browser] Connecting to ChromeDriver...")
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", chromeDriverPort))
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.chrome_driver_connect_failed", map[string]interface{}{"Error": err.Error()}))
	}

	hideWebdriverScript := `Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`
	if _, err := wd.ExecuteScript(hideWebdriverScript, nil); err != nil {
		wd.Quit()
		return nil, fmt.Errorf(i18n.TWithData("browser.hide_webdriver_failed", map[string]interface{}{"Error": err.Error()}))
	}

	secChUa, secChUaPlatform := generateRandomFingerprint()
	setPlatformScript := fmt.Sprintf(`Object.defineProperty(navigator, 'platform', {get: () => %s});`, secChUaPlatform)
	if _, err := wd.ExecuteScript(setPlatformScript, nil); err != nil {
		debugLog("[Browser] Failed to set platform fingerprint: %v", err)
	}

	debugLog("[Browser] Browser session created successfully (Sec-Ch-Ua: %s)", secChUa)
	return &Browser{
		wd:    wd,
		ownWD: true,
	}, nil
}

func (b *Browser) Close() {
	if b.ownWD && b.wd != nil {
		b.wd.Quit()
		b.ownWD = false
	}
}

func (b *Browser) Search(terms string, limit int) ([]SearchResult, error) {
	if err := b.wd.Get("https://duckduckgo.com/"); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.page_access_failed", map[string]interface{}{"Error": err.Error()}))
	}

	searchBox, err := b.wd.FindElement(selenium.ByName, "q")
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.search_box_not_found", map[string]interface{}{"Error": err.Error()}))
	}

	if err := searchBox.SendKeys(terms); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.search_input_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if err := searchBox.SendKeys(selenium.EnterKey); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.search_submit_failed", map[string]interface{}{"Error": err.Error()}))
	}

	time.Sleep(2 * time.Second)

	for i := 0; i < limit; i++ {
		searchMore, err := b.wd.FindElement(selenium.ByID, "more-results")
		if searchMore == nil || err != nil {
			break
		}

		searchMore.Click()
		debugPrintf("[Browser] Loading more results %d\n", i)
		time.Sleep(3 * time.Second)
	}

	var results []SearchResult
	articles, err := b.wd.FindElements(selenium.ByTagName, "article")
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.search_results_not_found", map[string]interface{}{"Error": err.Error()}))
	}

	for _, ele := range articles {
		headerEle, err := ele.FindElement(selenium.ByTagName, "h2")
		if err != nil {
			continue
		}

		linkEle, err := headerEle.FindElement(selenium.ByTagName, "a")
		if err != nil {
			continue
		}

		titleText, err := headerEle.Text()
		if err != nil {
			continue
		}

		link, _ := linkEle.GetAttribute("href")

		divEles, _ := ele.FindElements(selenium.ByCSSSelector, "div>span")
		if len(divEles) < 3 {
			continue
		}

		descText, err := divEles[2].Text()
		if err != nil {
			continue
		}

		results = append(results, SearchResult{
			Title:       titleText,
			Link:        link,
			Description: descText,
		})
	}

	return results, nil
}

func (b *Browser) Open(url string) (*OpenResult, error) {
	if err := b.wd.Get(url); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.page_access_failed", map[string]interface{}{"Error": err.Error()}))
	}

	_ = b.wd.Wait(func(wd selenium.WebDriver) (bool, error) {
		element, err := wd.FindElement(selenium.ByTagName, "body")
		return err == nil && element != nil, nil
	})

	text, err := b.wd.FindElements(selenium.ByTagName, "body")
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.page_text_failed", map[string]interface{}{"Error": err.Error()}))
	}
	if len(text) == 0 {
		return nil, fmt.Errorf(i18n.T("browser.page_body_not_found"))
	}

	content, err := text[0].Text()
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("browser.page_text_failed", map[string]interface{}{"Error": err.Error()}))
	}

	links, err := b.wd.FindElements(selenium.ByTagName, "a")
	if err != nil {
		debugLog("[Open] Failed to get links: %v", err)
		return &OpenResult{Content: content, Refs: []LinkRef{}}, nil
	}

	var refs []LinkRef
	for _, link := range links {
		href, err := link.GetAttribute("href")
		if err != nil || href == "" || href == "#" {
			continue
		}

		title, _ := link.Text()
		if title == "" {
			title, _ = link.GetAttribute("title")
		}
		if title == "" {
			title, _ = link.GetAttribute("aria-label")
		}

		refs = append(refs, LinkRef{
			Title: title,
			URL:   href,
		})
	}

	return &OpenResult{Content: content, Refs: refs}, nil
}

func randomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 11.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	}
	return userAgents[rand.Intn(len(userAgents))]
}

func randomWindowSize() string {
	windowSizes := []string{
		"1920,1080",
		"1366,768",
		"1536,864",
		"2560,1440",
		"1440,900",
		"1600,900",
	}
	return windowSizes[rand.Intn(len(windowSizes))]
}

func generateRandomFingerprint() (secChUa, secChUaPlatform string) {
	chromeVersions := []string{
		`"Chromium";v="129", "Google Chrome";v="129", "Not=A?Brand";v="99"`,
		`"Chromium";v="128", "Google Chrome";v="128", "Not=A?Brand";v="99"`,
		`"Chromium";v="127", "Google Chrome";v="127", "Not=A?Brand";v="99"`,
	}
	platforms := []string{`"Windows"`, `"macOS"`}
	return chromeVersions[rand.Intn(len(chromeVersions))],
		platforms[rand.Intn(len(platforms))]
}

func findChromeDriver() string {
	if path, err := exec.LookPath("chromedriver"); err == nil {
		return path
	}

	paths := []string{
		"/usr/local/bin/chromedriver",
		"/opt/homebrew/bin/chromedriver",
		"/usr/bin/chromedriver",
		"/Applications/chromedriver",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
