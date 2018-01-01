package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"time"

	"github.com/mmcdole/gofeed"
)

type configuration struct {
	EmailTo   string
	EmailFrom string
	EmailPass string
	Twitter   string
	TwSecret  string
	Rss       []struct {
		Site string
		Tag  string
	}
	Tags map[string]int
}

func decode(conf configuration) map[string][]string {
	output := make(map[string][]string)
	fp := gofeed.NewParser()
	today := time.Now()
	for _, xx := range conf.Rss {
		feed, _ := fp.ParseURL(xx.Site)
		duration, ok := conf.Tags[xx.Tag]
		if !ok {
			duration = 1
		}
		tag := xx.Tag
		if tag == "" {
			tag = feed.Title
		}
		if today.YearDay()%duration != 0 {
			continue
		}
		if _, ok := output[tag]; !ok {
			output[tag] = make([]string, 0)
		}
		for _, yy := range feed.Items {
			localTime, err := time.Parse(time.RFC1123, yy.Published)
			if err != nil {
				localTime, err = time.Parse(time.RFC1123Z, yy.Published)
				if err != nil {
					log.Fatal(err)
				}
			}
			diff := today.Sub(localTime)
			if diff <= time.Duration(float64(duration)*1.1*60*60*24)*time.Second {
				output[tag] = append(output[tag], fmt.Sprintf("<a href=\"%s\">%s</a><br>%s<br><br>\n", yy.Link, yy.Title, yy.Description))
			}
		}
	}
	return output
}

func collect(conf configuration, kvs map[string][]string) {
	auth := smtp.PlainAuth("", conf.EmailFrom, conf.EmailPass, "smtp.gmail.com")
	for xx, yy := range kvs {
		if len(yy) == 0 {
			continue
		}
		to := []string{conf.EmailTo}
		msg := "To:  " + conf.EmailTo + "\r\n" +
			"Subject: " + xx + "!\n" +
			"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n" +
			`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"` +
			`"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">  <html><body>`

		for _, zz := range yy {
			msg = msg + zz + "\n\n"
		}

		msg = msg + "</body></html>"
		err := smtp.SendMail("smtp.gmail.com:587", auth, conf.EmailTo, to, []byte(msg))
		if err != nil {
			log.Fatal(err)
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

func main() {
	config := flag.String("config", "config.json", "config location")
	flag.Parse()
	conf := parseConfig(*config)
	kvs := decode(conf)
	collect(conf, kvs)
}
