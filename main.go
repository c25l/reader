package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/araddon/dateparse"
	"github.com/mmcdole/gofeed"
)

var (
	OutLog = "** Run Log\n"
)

type rss struct {
	Site  string
	Limit int
}

type configuration struct {
	OutputPath string
	Rss        []rss
}

func request(f *gofeed.Parser, feedURL string) (*gofeed.Feed, error) {
	client := http.Client{}
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	req.Header.Set("User-Agent", "Golang_Bugwilla")

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)

		return nil, err
	}
	if resp != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, gofeed.HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	return f.Parse(resp.Body)
}

func TokenReplacer(in []byte) (out []byte) {
	if len(in) < 4 {
		return
	}
	if (in[1] == 'a') && (in[2] == ' ') {
		return in
	}
	if (in[1] == '/') && (in[2] == 'a') && (len(in) == 4) {
		return in
	}
	return
}

func decode(feeds []rss) map[string][]string {
	output := make(map[string][]string)
	brackets, _ := regexp.Compile(`[\[\]]*`)
	links, _ := regexp.Compile(`<a href="([^">]*)">([^<]*)</a>`)
	quotes, _ := regexp.Compile(`<blockquote>(.*)</blockquote>`)
	tokens, _ := regexp.Compile(`<[^>]*>`)

	fp := gofeed.NewParser()
	today := time.Now()
	for _, xx := range feeds {

		feed, err := request(fp, xx.Site)
		if err != nil {
			logger(xx.Site, err)
			continue
		}
		tag := feed.Title
		if _, ok := output[tag]; !ok {
			output[tag] = make([]string, 0)
		}
		offset := 0
		items := len(feed.Items)
		if (xx.Limit != 0) && (xx.Limit < items) {
			items = xx.Limit
		}
		for ii := 0; ii < items; ii++ {
			yy := feed.Items[ii]
			useDate := true
			localTime, err := dateparse.ParseAny(yy.Published)
			if err != nil {
				useDate = false
			}
			diff := today.Sub(localTime)
			offset++
			condition := false
			if useDate {
				condition = diff <= time.Duration(1.0*60*60*24)*time.Second
			} else {
				condition = offset < 2
			}
			usetext := brackets.ReplaceAll([]byte(yy.Description), []byte(""))
			usetext = tokens.ReplaceAllFunc(usetext, TokenReplacer)

			usetext = links.ReplaceAll(usetext, []byte(`[[$1][$2]]`))
			usetext = quotes.ReplaceAll(usetext, []byte("\n#+BEGIN_QUOTE\n$1\n#+END_QUOTE\n"))
			if len(usetext) > 2000 {
				usetext = usetext[:1000]
			}
			if condition {
				output[tag] = append(output[tag], fmt.Sprintf("** TODO %s - [[%s][%s]]\n%s\n", tag, yy.Link, yy.Title, usetext))
			}
		}
		logger(fmt.Sprintf("Feed %s found %d items", feed.Title, len(output[tag])))
	}
	return output
}

func parseConfig(loc string) configuration {
	file, err := os.Open(loc)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	jdec := json.NewDecoder(file)
	var conf configuration
	err = jdec.Decode(&conf)
	if err != nil {
		log.Fatal(err)
	}
	return conf
}

func emitOrg(kvs map[string][]string, dest string) {
	f, err := os.OpenFile(dest, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now()
	year, month, day := now.Date()
	dow := now.Weekday().String()
	f.Write([]byte(fmt.Sprintf("* %d-%02d-%02d %s\n", year, int(month), day, dow)))
	for _, x := range kvs {
		for _, val := range x {
			if _, err := f.Write([]byte(val)); err != nil {
				log.Fatal(err)
			}
		}
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

func logger(args ...interface{}) {
	OutLog = OutLog + "*** " + fmt.Sprint(args...) + "\n"
}

func main() {
	config := flag.String("config", "config.json", "config location")
	flag.Parse()
	logger("starting at", time.Now().String())
	conf := parseConfig(*config)
	kvs := decode(conf.Rss)
	kvs["runlog"] = []string{OutLog}
	emitOrg(kvs, conf.OutputPath)
}
