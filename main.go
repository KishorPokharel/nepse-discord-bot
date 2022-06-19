package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

type company struct {
	ID     int    `json:"id"`
	Symbol string `json:"symbol"`
}

func main() {
	if os.Getenv("TOKEN") == "" {
		log.Fatal("TOKEN not set")
	}

	f, err := os.Open("companies.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var companies []*company
	if err := json.NewDecoder(f).Decode(&companies); err != nil {
		log.Fatal("could not decode(): ", err)
	}

	token := os.Getenv("TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("could not create new discord session: ", err)
	}
	dg.AddHandler(messageCreate(companies))
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-sc
	dg.Close()
}

func messageCreate(companies []*company) func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		if strings.HasPrefix(m.Content, "!price ") {
			company := strings.TrimPrefix(m.Content, "!price ")
			code := findCode(companies, company)
			if code == -1 {
				s.ChannelMessageSend(m.ChannelID, "name not found in the list")
				return
			}
			price := fetchPrice(code)
			s.ChannelMessageSend(m.ChannelID, "Price: Rs. "+price)
		}
	}
}

func findCode(companies []*company, symb string) int {
	for _, company := range companies {
		if company.Symbol == strings.ToUpper(symb) {
			return company.ID
		}
	}
	return -1
}

func fetchPrice(code int) string {
	errfetch := "err: could not fetch price"
	url := fmt.Sprintf("http://www.nepalstock.com/company/display/%d", code)
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Println("could not Get(): ", err)
		return errfetch
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Println(err)
		return errfetch
	}
	tableSelection := doc.Find(".my-table.table").First()
	var found bool
	var price string
	tableSelection.Find("tr").Each(func(i int, s *goquery.Selection) {
		s.Find("td").Each(func(i int, s *goquery.Selection) {
			if found {
				price = s.Text()
				found = false
			}
			if s.Text() == "Last Traded Price (Rs.)" {
				found = true
			}
		})
	})
	if price != "" {
		return price
	}
	return errfetch
}
