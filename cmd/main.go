package main

import (
	"DouBanReptile/internal/gui"
	"DouBanReptile/internal/log"
	"DouBanReptile/internal/markdown"
	"DouBanReptile/internal/request"
	"DouBanReptile/internal/scheduler"
	"DouBanReptile/internal/xpath"
	"flag"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dispatcher scheduler.Dispatcher
var file *os.File
var priceCompile = regexp.MustCompile(`\b\d{4}\b`)
var dataArray = make(markdown.DataArray, 0)
var doesNotAppear []string
var groupURL *string
var maxPrice *int
var isIncludeNoContentPrice *bool

func main() {
	//handleArgs()
	gui.Open(func(p gui.Preference) {
		doesNotAppear = p.ExcludeKeyArray
		isIncludeNoContentPrice = &p.IncludeNoContentPriceCheck
		maxPrice = &p.MaxPrice
		groupURL = &p.GroupEntityURL

		headerMap := make(map[string]string)
		headerMap["User-Agent"] = request.UserAgentPCChrome

		dispatcher = scheduler.Dispatcher{
			BaseUrl: "https://www.douban.com",
			Headers: headerMap,
		}
		dispatcher.Init2(
			*groupURL,
			`//td[@class='title']/a`,
			each,
			time.Millisecond*500,
			&scheduler.PaginationRange{StartSize: 0, EndSize: p.MaxPage * 25, EveryAdd: 25})

		write2File()
	})
}

func handleArgs() {
	excludeKey := flag.String("e", "限女", "排除关键字用|分隔")
	groupURL = flag.String("g", "/group/554566/discussion?start=%d", "设置豆瓣群组链接")
	maxPrice = flag.Int("m", 1500, "设置最大价格")
	isIncludeNoContentPrice = flag.Bool("i", false, "设置包含不带价格的")
	flag.Parse()
	excludeKeyArray := strings.Split(*excludeKey, "|")
	for _, key := range excludeKeyArray {
		doesNotAppear = append(doesNotAppear, key)
	}
	logger := log.GetImpl()
	logger.Printf("群组：%s\n", *groupURL)
	logger.Printf("排除关键字：%s\n", doesNotAppear)
	logger.Printf("最大价格：%d\n", *maxPrice)
	logger.Printf("包含不带价格的：%t\n", *isIncludeNoContentPrice)
}

func write2File() {
	var err error
	file, err = os.Create("爬取结果.md")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()
	logger := log.GetImpl()
	logger.Printf("共爬取条数: %d", len(dataArray))
	logger.Printf("写入文件中....")
	write([]byte(dataArray.String()))
	logger.Printf("文件写入成功，请在本EXE目录下找【%s】文件", file.Name())
}

func write(b []byte) {
	if _, err := file.Write(b); err != nil {
		panic(err)
	}
}

func each(nodes xpath.Nodes, request request.Data) {
	hrefs := nodes.Attr("href")
	titles := nodes.Attr("title")
	for index, href := range hrefs {
		if isExcludeContent(titles[index]) {
			continue
		}
		price := getPriceFromString(titles[index])
		if price != 0 && price <= (*maxPrice) {
			dispatcher.Add(href, `//div[@class="article"]`, content)
		} else if price == 0 && *isIncludeNoContentPrice {
			dispatcher.Add(href, `//div[@class="article"]`, content)
		}
	}
}

func isExcludeContent(content string) bool {
	for _, key := range doesNotAppear {
		if strings.Contains(content, key) {
			return true
		}
	}
	return false
}

func getPriceFromString(title string) int {
	priceArray := priceCompile.FindAllString(title, -1)
	if len(priceArray) != 0 {
		price, e := strconv.Atoi(priceArray[0])
		if e != nil {
			log.GetImpl().Printf("Transform Error %s", e.Error())
			panic(e)
		}
		return price
	} else {
		return 0
	}
}

func content(nodes xpath.Nodes, request request.Data) {
	for _, node := range nodes {
		// 处理标题
		title := handleTitle(node)
		// 处理内容
		content := handleContent(node)
		if strings.Contains(content, "限女") {
			continue
		}
		// 处理图片
		imgArray := handleImages(node)
		// 处理时间
		timeStr := handleTime(node)

		dataArray.Append(markdown.Data{
			TimeString: timeStr,
			Time:       markdown.String2Time(timeStr),
			Title:      format(title),
			Price:      getPriceFromString(title),
			Link:       request.Url,
			Content:    content,
			Images:     imgArray,
		})
	}
}

func handleTime(node *html.Node) string {
	timeNode := htmlquery.FindOne(node, `//span[@class="color-green"]`)
	timeStr := htmlquery.InnerText(timeNode)
	return timeStr
}

func handleImages(node *html.Node) []string {
	imgs := htmlquery.Find(node, `//div[@class="image-wrapper"]//img`)
	imgArray := make([]string, 0)
	if len(imgs) != 0 {
		for _, img := range imgs {
			imgArray = append(imgArray, htmlquery.SelectAttr(img, "src"))
		}
	}
	return imgArray
}

func handleContent(node *html.Node) string {
	contentNode := htmlquery.FindOne(node, `//td[@class="topic-content"]`)
	if contentNode == nil {
		contentNode = htmlquery.FindOne(node, `//div[@class="topic-richtext"]`)
	}
	content := htmlquery.InnerText(contentNode)
	return content
}

func handleTitle(node *html.Node) string {
	titleNode := htmlquery.FindOne(node, `//td[@class="tablecc"]`)
	if titleNode == nil {
		titleNode = htmlquery.FindOne(node, `//h1`)
	}
	title := htmlquery.InnerText(titleNode)
	if strings.Contains(title, "标题") {
		title = string([]rune(title)[3:])
	}
	return title
}

func format(str string) string {
	return strings.TrimSpace(str)
}
