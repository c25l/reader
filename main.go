package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/mmcdole/gofeed"
)

var (
	OutLog = ""
)

type rss struct {
	Site  string
	Tag   string
	Limit int
}

type configuration struct {
	EmailTo               string
	EmailFrom             string
	EmailPass             string
	TwitterConsumerKey    string
	TwitterConsumerSecret string
	TwitterAccessToken    string
	TwitterAccessSecret   string
	Rss                   []rss
	Tags                  map[string]int
}

func request(f *gofeed.Parser, feedURL string) (*gofeed.Feed, error) {
	client := http.Client{}
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("User-Agent", "Golang_Bugwilla")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
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

func decode(feeds []rss, tags map[string]int) map[string][]string {
	output := make(map[string][]string)
	fp := gofeed.NewParser()
	today := time.Now()
	for _, xx := range feeds {
		duration, ok := tags[xx.Tag]
		if !ok {
			duration = 1
		}
		if today.YearDay()%duration != 0 {
			logger("wrong day for", xx.Site)
			continue
		}
		tag := xx.Tag
		feed, err := request(fp, xx.Site)
		if err != nil {
			logger(xx.Site, err)
			continue
		}
		if tag == "" {
			tag = feed.Title
		}
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
			localTime, err := time.Parse(time.RFC1123, yy.Published)
			useDate := true
			if err != nil {
				localTime, err = time.Parse(time.RFC1123Z, yy.Published)
				if err != nil {
					useDate = false
				}
			}
			diff := today.Sub(localTime)
			offset++
			condition := false
			if useDate {
				condition = diff <= time.Duration(float64(duration)*1.1*60*60*24)*time.Second
			} else {
				condition = offset < 20
			}
			usetext := yy.Description
			if len(usetext) > 2000 {
				usetext = usetext[:1000]
			}
			if condition {
				output[tag] = append(output[tag], fmt.Sprintf("<a href=\"%s\">%s</a><br>%s<br><br>\n", yy.Link, yy.Title, usetext))
			}
		}
		logger(fmt.Sprintf("Feed %s found %d items", feed.Title, len(output[tag])))
	}
	return output
}

func pullTwitter(ck, cs, ak, as string) []string {
	config := oauth1.NewConfig(ck, cs)
	token := oauth1.NewToken(ak, as)
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)

	// Home Timeline
	tweets, resp, err := client.Timelines.HomeTimeline(&twitter.HomeTimelineParams{
		Count: 100,
	})
	output := make([]string, 0)
	if err != nil {
		logger("twitter error", resp, err)
		return output
	}
	today := time.Now()
	for _, tw := range tweets {
		ctime, err := tw.CreatedAtTime()
		if (err == nil) && (today.Sub(ctime) < time.Duration(1.1*60*60*24)*time.Second) {
			link := fmt.Sprintf("<a href=\"https://twitter.com/%s/status/%s\">%s</a>", tw.User.ScreenName, tw.IDStr, tw.User.Name)
			output = append(output, fmt.Sprintf("%s : %s <br><br>", link, tw.Text))
		}
	}
	logger(fmt.Sprintf("found %d tweets", len(output)))
	return output
}

func sendEmail(from, to, pass string, kvs map[string][]string) {
	auth := smtp.PlainAuth("", from, pass, "smtp.gmail.com")
	for xx, yy := range kvs {
		if len(yy) == 0 {
			continue
		}
		msg := "To:  " + to + "\r\n" +
			"Subject: " + xx + "\r\n" +
			"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n" +
			`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"` +
			`"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">  <html><body>`

		for _, zz := range yy {
			msg = msg + zz + "\n\n"
		}

		msg = msg + "</body></html>"
		err := smtp.SendMail("smtp.gmail.com:587", auth, to, []string{to}, []byte(msg))
		if err != nil {
			log.Fatal(OutLog, err)
		}

	}
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

func logger(args ...interface{}) {
	OutLog = OutLog + fmt.Sprint(args) + "<br>\n"
}

func main() {
	config := flag.String("config", "config.json", "config location")
	flag.Parse()
	logger("bugwilla starting at", time.Now().String())
	conf := parseConfig(*config)
	kvs := decode(conf.Rss, conf.Tags)
	if conf.TwitterAccessSecret != "" {
		twitter := pullTwitter(conf.TwitterConsumerKey, conf.TwitterConsumerSecret,
			conf.TwitterAccessToken, conf.TwitterAccessSecret)
		kvs["twitter"] = twitter
	}
	kvs["bugwilla runlog"] = []string{OutLog}
	sendEmail(conf.EmailFrom, conf.EmailTo, conf.EmailPass, kvs)

}
