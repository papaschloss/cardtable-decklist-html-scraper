package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {

	e := echo.New()

	// override AllowOrigins for CORS by setting an environment variable CORS_HOSTS.
	// separate the URLs with ;
	hosts, ok := os.LookupEnv("CORS_HOSTS");
	hostsArr := []string{};
	if ok {
		hostsArr = strings.Split(hosts, ";")
	} else {
		hostsArr = []string{"*.middle-earth.house", "https://card-table.app"}
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: hostsArr,
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	type deckInfo struct {
		Name  string
		Hero  string
		By    string
		Likes int
	}

	type searchInfo struct {
		StatusCode int
		Decks      map[string]*deckInfo
	}

	type variables struct {
		DeckId int `json:"deckId"`
	}

	type rangersQuery struct {
		OperationName string    `json:"operationName"`
		Variables     variables `json:"variables"`
		Query         string    `json:"query"`
	}

	e.GET("/rangersproxy", func(c echo.Context) error {

		query := c.Request().URL.Query()

		deckIdString := query.Get("deckId")
		deckId, queryErr := strconv.Atoi(deckIdString)

		if queryErr != nil {
			return queryErr
		}

		//Encode the data
		postBody, _ := json.Marshal(rangersQuery{
			OperationName: "getDeck",
			Variables:     variables{DeckId: deckId},
			Query:         "query getDeck($deckId: Int!) {\n  deck: rangers_deck_by_pk(id: $deckId) {\n    ...DeckDetail\n    __typename\n  }\n}\n\nfragment DeckDetail on rangers_deck {\n  ...Deck\n  copy_count\n  comment_count\n  like_count\n  liked_by_user\n  original_deck {\n    deck {\n      id\n      name\n      user {\n        id\n        handle\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n  campaign {\n    id\n    name\n    rewards\n    latest_decks {\n      deck {\n        id\n        slots\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n  user {\n    handle\n    __typename\n  }\n  comments(order_by: {created_at: asc}, limit: 5) {\n    ...BasicDeckComment\n    __typename\n  }\n  __typename\n}\n\nfragment Deck on rangers_deck {\n  id\n  user_id\n  slots\n  side_slots\n  extra_slots\n  version\n  name\n  description\n  awa\n  spi\n  fit\n  foc\n  created_at\n  updated_at\n  meta\n  user {\n    ...UserInfo\n    __typename\n  }\n  published\n  previous_deck {\n    id\n    meta\n    slots\n    side_slots\n    version\n    __typename\n  }\n  next_deck {\n    id\n    meta\n    slots\n    side_slots\n    version\n    __typename\n  }\n  __typename\n}\n\nfragment UserInfo on rangers_users {\n  id\n  handle\n  __typename\n}\n\nfragment BasicDeckComment on rangers_comment {\n  id\n  user {\n    ...UserInfo\n    __typename\n  }\n  text\n  created_at\n  updated_at\n  response_count\n  comment_id\n  __typename\n}",
		})

		responseBody := bytes.NewBuffer(postBody)
		resp, err := http.Post("https://gapi.rangersdb.com/v1/graphql", "application/json", responseBody)

		if err != nil {
			return err
		}

		defer resp.Body.Close()
		var data map[string]interface{}

		if resp.StatusCode == http.StatusOK {

			bodyDecodeErr := json.NewDecoder(resp.Body).Decode(&data)
			if bodyDecodeErr != nil {
				log.Println(bodyDecodeErr)
				return bodyDecodeErr
			}

			log.Println("")
			temp, _ := json.Marshal(data)
			log.Println(string(temp))
		}

		return c.JSON(http.StatusOK, data)
	})

	e.GET("/", func(c echo.Context) error {
		query := c.Request().URL.Query()

		co := colly.NewCollector(
			colly.AllowedDomains("marvelcdb.com", "ringsdb.com"),
		)

		s := &searchInfo{Decks: make(map[string]*deckInfo)}

		parseData := func(h *colly.HTMLElement) {
			link := h.DOM.Find("a[href^=\"/decklist/view/\"]").First()

			if len(link.Text()) == 0 {
				return
			}
			deckName := link.Text()
			id := strings.Split(link.AttrOr("href", "nothing/to/see//"), "/")[3]
			if len(id) == 0 {
				return
			}

			s.Decks[id] = &deckInfo{Name: deckName, Hero: "", By: "", Likes: 0}

			hero := h.DOM.Find(".fg-hero").First()
			if len(hero.Text()) > 0 {
				s.Decks[id].Hero = hero.Text()
			}

			by := h.DOM.Find("a[href^=\"/user/profile/\"]").First()
			if len(by.Text()) > 0 {
				s.Decks[id].By = by.Text()
			}

			likes := h.DOM.Find(".num").First()
			if len(likes.Text()) > 0 {
				likesInt, intCovertErr := strconv.Atoi(likes.Text())
				if intCovertErr == nil {
					s.Decks[id].Likes = likesInt
				}
			}
		}

		co.OnHTML(".table > tbody > tr", func(h *colly.HTMLElement) {
			parseData(h)
		})

		co.OnHTML(".decklists > .box", func(h *colly.HTMLElement) {
			parseData(h)

		})

		co.OnResponse(func(r *colly.Response) {
			log.Println("response received, status: ", r.StatusCode)
			s.StatusCode = r.StatusCode
		})

		co.OnError(func(r *colly.Response, err error) {
			log.Println("error received, status: ", r.StatusCode, ", error: ", err)
			s.StatusCode = r.StatusCode
		})

		fmt.Printf("visiting uri: %s", query.Get("uri"))
		fmt.Println()
		co.Visit(query.Get("uri"))

		pretty, err := json.MarshalIndent(s, "", "    ")
		if err != nil {
			return c.HTML(http.StatusInternalServerError, "")
		}
		println(string(pretty))

		return c.JSON(http.StatusOK, s)
	})

	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct{ Status string }{Status: "OK"})
	})

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8281"
	}

	e.Logger.Fatal(e.Start(":" + httpPort))
}
