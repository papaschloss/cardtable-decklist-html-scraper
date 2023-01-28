package main

import (
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

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*.middle-earth.house"},
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

	e.GET("/", func(c echo.Context) error {
		query := c.Request().URL.Query()

		co := colly.NewCollector(
			colly.AllowedDomains("marvelcdb.com", "ringsdb.com"),
		)

		s := &searchInfo{Decks: make(map[string]*deckInfo)}

		co.OnHTML("", func(h *colly.HTMLElement) {
			// deckName := h.Text
			// id := strings.Split(h.Attr("href"), "/")[3]
			// s.Decks[id] = strings.Trim(deckName, " ")
		})

		co.OnHTML(".decklists > .box", func(h *colly.HTMLElement) {
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

		jsonResult, err := json.Marshal(s)
		if err != nil {
			return c.HTML(http.StatusInternalServerError, "")
		}
		pretty, err := json.MarshalIndent(s, "", "    ")
		if err != nil {
			return c.HTML(http.StatusInternalServerError, "")
		}
		println(string(pretty))

		return c.JSON(http.StatusOK, jsonResult)
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
