package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
	Site string
	Tag  string
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
		feed, err := fp.ParseURL(xx.Site)
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
		for _, yy := range feed.Items {
			localTime, err := time.Parse(time.RFC1123, yy.Published)
			if err != nil {
				localTime, err = time.Parse(time.RFC1123Z, yy.Published)
				if err != nil {
					logger(tag, err)
				}
			}
			diff := today.Sub(localTime)
			if diff <= time.Duration(float64(duration)*1.1*60*60*24)*time.Second {
				output[tag] = append(output[tag], fmt.Sprintf("<a href=\"%s\">%s</a><br>%s<br><br>\n", yy.Link, yy.Title, yy.Description))
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
	logger("harvester starting at", time.Now().String())
	conf := parseConfig(*config)
	kvs := decode(conf.Rss, conf.Tags)
	if conf.TwitterAccessSecret != "" {
		twitter := pullTwitter(conf.TwitterConsumerKey, conf.TwitterConsumerSecret,
			conf.TwitterAccessToken, conf.TwitterAccessSecret)
		kvs["twitter"] = twitter
	}
	kvs["harvester runlog"] = []string{OutLog}
	sendEmail(conf.EmailFrom, conf.EmailTo, conf.EmailPass, kvs)

}
