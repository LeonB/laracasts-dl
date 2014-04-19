package main

// https://github.com/thbar/golang-playground/blob/master/download-files.go

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/cheggaaa/pb"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func usage() {
	banner := fmt.Sprintf("%v USERNAME PASSWORD [DIRECTORY]", os.Args[0])
	log.Fatal(banner)
}

func parse_options() config {
	if len(os.Args) < 3 {
		usage()
	}

	config := config{
		Username:  os.Args[1],
		Password:  os.Args[2],
	}

	if len(os.Args) > 3 {
		config.Directory = os.Args[3]
	} else {
		config.Directory = "."
	}

	return config
}

func main() {
	config := parse_options()
	scraper := NewScraper(config)

	// First login to get more data in views
	log.Println("Logging in")
	err := scraper.Login()

	if err != nil {
		log.Fatal(err)
	}

	lessons, err := scraper.GetAvailableLessons()
	log.Printf("Found %v lessons", len(lessons))

	if err != nil {
		log.Fatal(err)
	}

	for i, lesson := range lessons {
		log.Printf("Downloading lesson %v/%v (%v)", i+1, len(lessons), lesson.Name)
		err = scraper.DownloadLesson(lesson)
	}
}

type config struct {
	Username  string
	Password  string
	Directory string
}

type scraper struct {
	// Name string
	BaseUrl string
	Client  http.Client
	Username string
	Password string
	Directory string
}

type Lesson struct {
	Id   int
	Name string
	Url  string
}

func (l *Lesson) GetFilename(contentType string) (string, error) {
	pieces := strings.Split(l.Url, "/")
	basename := pieces[len(pieces)-1]

	pieces = strings.Split(contentType, "/")
	extension := pieces[len(pieces)-1]

	return basename + "." + extension, nil
}

func NewScraper(config config) scraper {
	s := scraper{}
	s.BaseUrl = "https://laracasts.com"

	s.Username = config.Username
	s.Password = config.Password
	s.Directory = config.Directory

	jar, _ := cookiejar.New(nil)
	s.Client = http.Client{
		Jar: jar,
	}
	return s
}

func (s *scraper) GetAvailableLessons() ([]Lesson, error) {
	episodes := []Lesson{}

	url := s.BaseUrl + "/all"
	resp, err := s.Client.Get(url)

	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	links := doc.Find(".container a[href*='/lessons/']")
	links.Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		name, _ := s.Html()

		p := s.Parent()
		input := p.Find("[name='lesson-id']")
		str, _ := input.Attr("value")
		lessonId, _ := strconv.Atoi(str)

		lesson := Lesson{}
		lesson.Id = lessonId
		lesson.Url = href
		lesson.Name = name

		episodes = append(episodes, lesson)
	})

	return episodes, nil
}

func (s *scraper) Login() error {
	u := s.BaseUrl + "/sessions"
	resp, err := s.Client.PostForm(u,
		url.Values{"email": {s.Username}, "password": {s.Password}})
	defer resp.Body.Close()

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("Login return wrong status code: %v, expected %v. Is your username/password correct?",
			resp.StatusCode, 200)
	}

	return nil
}

func (s *scraper) DownloadLesson(lesson Lesson) error {
	url := s.BaseUrl + "/downloads/" + strconv.Itoa(lesson.Id) + "?type=lesson"

	resp, err := s.Client.Get(url)
	defer resp.Body.Close()

	if err != nil {
		log.Println("Error while downloading", url, "-", err)
		return nil
	}

	headers := resp.Header
	filename, err := lesson.GetFilename(headers["Content-Type"][0])
	filename = s.Directory + "/" + filename

	// Create new progressbar
	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.ShowSpeed = true
	bar.Start()

	// TODO: check file existence first with io.IsExist
	dest, err := os.Create(filename)
	if err != nil {
		log.Println("Error while creating", filename, "-", err)
		return nil
	}
	defer dest.Close()

	// create multi writer
	writer := io.MultiWriter(dest, bar)

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		log.Println("Error while downloading", url, "-", err)
		return nil
	}

	return nil
}
