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
	// Check if username, password and directory (opt) is set
	config := parse_options()
	scraper := NewScraper(config)

	// First login to get more data in views
	log.Println("Logging in")
	err := scraper.Login()

	if err != nil {
		log.Fatal(err)
	}

	// Find all lessons from /all
	lessons, err := scraper.GetAvailableLessons()
	log.Printf("Found %v lessons", len(lessons))

	if err != nil {
		log.Fatal(err)
	}

	// Loop all lessons and download them
	for i, lesson := range lessons {
		log.Printf("Checking lesson %v/%v (%v)", i+1, len(lessons), lesson.Name)
		err = scraper.DownloadLesson(lesson)
		if err != nil {
			log.Printf("skiping: %v", err)
		}
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

// Determine what the proper filename for a lesson should be
func (l *Lesson) GetFilename(contentType string) (string, error) {
	pieces := strings.Split(l.Url, "/")
	basename := pieces[len(pieces)-1]

	pieces = strings.Split(contentType, "/")
	extension := pieces[len(pieces)-1]

	return fmt.Sprintf("%v-%v.%v", strconv.Itoa(l.Id), basename, extension), nil
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

// Find all lesson on /all
// To get the lessonId you have to be logged in
func (s *scraper) GetAvailableLessons() ([]Lesson, error) {
	episodes := []Lesson{}

	url := s.BaseUrl + "/all"
	resp, err := s.Client.Get(url)

	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromResponse(resp)

	// Find all links to lessons
	links := doc.Find(".container a[href*='/lessons/']")
	links.Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		name, _ := s.Html()

		// Find the lessonId
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

// Login to laracasts
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

// Download a specific lesson and put it in a directory
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
	path := s.Directory + "/" + filename

	// Open the destination, return an error when the file already exists
	dest, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	defer dest.Close()

	if err != nil {
		// Nog a "already exists" error: blow up
		if !os.IsExist(err) {
			log.Fatal(err)
		}

		// OpenFile() + os.O_EXCL doesn't return a File
		dest, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
		defer dest.Close()

		// Check if the video sizes online and local are the same
		fileInfo, _ := dest.Stat()
		if (fileInfo.Size() == resp.ContentLength) {
			return fmt.Errorf("%v already exists (and is the same size)", filename)
		}
		return nil
	}

	// Create new progressbar
	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.ShowSpeed = true
	bar.Start()

	// create multi writer
	writer := io.MultiWriter(dest, bar)

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		log.Println("Error while downloading", url, "-", err)
		return nil
	}

	return nil
}
