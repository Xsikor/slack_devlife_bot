package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"
)

const (
	KB         = 1024
	MB         = KB * KB
	getURL     = "https://developerslife.ru/latest/0?json=true"
	MaxGifSize = 3 * MB
)

var (
	tmpIDFile    = path.Join(os.TempDir(), "devlife.id")
	postURL = ""
	slackChannel = ""
	waitTime     int
)

func init() {
	// flag.StringVar(&slackChannel, "chan", "", "#channel or @user direction")
	// flag.StringVar(&postURL, "hook", "", "webhook url for post messages")

	flag.IntVar(&waitTime, "sleep", 2, "sleep time before recheck in minute")
	flag.Parse()

	slackChannel = os.Getenv("SLACK_CHAN")
	postURL = os.Getenv("SLACK_WEBHOOK_URL")

	if postURL == "" || slackChannel == "" {
		fmt.Println("Please set envirement SLACK_CHAN and SLACK_WEBHOOK_URL")
		os.Exit(0)
	}
}

type attachments struct {
	Fallback string `json:"fallback"`
	Text     string `json:"text"`
	ImageUrl string `json:"image_url"`
	Footer	string `json:"footer"`
}

type DevLifeJSON struct {
	Result []struct {
		ID          int64
		Description string
		Votes       int64
		Date        string `json:"date"`
		GifURL      string `json:"gifURL"`
		GifSize     int64  `json:"gifSize"`
	}
}

func main() {
	c := make(chan os.Signal, 0)
	tickTimer := time.NewTicker(time.Minute * time.Duration(waitTime))
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go tick()
	for {
		select {
		case s := <-c:
			log.Println("\nExit by", s.String())
			os.Exit(0)

		case <-tickTimer.C:
			go tick()
		}
	}
}

func tick() (err error) {
	out, err := GetNewPosts()
	if err != nil {
		log.Println("Can't get new posts", err)
		return
	}

	data := DevLifeJSON{}
	err = json.Unmarshal(out, &data)
	if err != nil {
		log.Println("Can't parse responce json", err)
		return
	}

	processPosts(data)

	date, err := time.Parse("Jan 2, 2006 15:04:05 PM", data.Result[0].Date)
	if err != nil {
		log.Println("Can't parse date", err)
		return
	}

	err = UpdateLastDate(date.Unix())
	if err != nil {
		log.Println("Can't update last date time", err)
		return
	}
	return
}

func processPosts(data DevLifeJSON) {
	lastDate := GetLastDate()
	for i := len(data.Result) - 1; i >= 0; i-- {
		v := data.Result[i]
		date, err := time.Parse("Jan 2, 2006 15:04:05 PM", v.Date)
		if err != nil {
			log.Println("Can't parse date", v.Date, err)
			continue
		}

		if lastDate >= date.Unix() || v.GifSize > MaxGifSize || v.Votes < 0 {
			log.Println("Skip", v.ID)
			continue
		}

		msg := attachments{
			Text:     v.Description,
			Fallback: v.Description,
			ImageUrl: v.GifURL,
			Footer: fmt.Sprintf("https://developerslife.ru/%d", v.ID),
		}
		err = SendToSlack(msg)
		log.Println("Send to slack", v.ID, err)
	}
}

func GetLastDate() int64 {
	f, err := os.Open(tmpIDFile)
	if os.IsNotExist(err) {
		return 0
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println("Can't read tmpIDFile file", err)
		return 0
	}

	i, err := strconv.ParseInt(string(b), 16, 64)
	if err != nil {
		log.Println("Can't parse int from string", err)
		return 0
	}

	return i
}

func UpdateLastDate(id int64) error {
	st := fmt.Sprintf("%x", id)
	return ioutil.WriteFile(tmpIDFile, []byte(st), 0777)
}

func GetNewPosts() (out []byte, err error) {
	resp, err := http.Get(getURL)
	if err != nil {
		return
	}

	return ioutil.ReadAll(resp.Body)
}

func SendToSlack(attach attachments) error {
	var msg = map[string]interface{}{
		"attachments": []attachments{
			attach,
		},
	}

	if slackChannel != "" {
		msg["channel"] = slackChannel
	}

	b, err := json.Marshal(msg)
	if err != nil {
		log.Println("Cant marshal msg", err, string(b))
		return err
	}
	resp, err := http.Post(postURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		log.Println(err)
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return err
	}

	if string(body) != "ok" {
		log.Println("Post to slack error", string(body), err, string(b))
	}

	return err
}
