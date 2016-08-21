package main

// https://github.com/thbar/golang-playground/blob/master/download-files.go

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"github.com/PuerkitoBio/goquery"
	"github.com/cheggaaa/pb"
)

func usage() {
	banner := fmt.Sprintf("%v USERNAME PASSWORD [DIRECTORY]", os.Args[0])
	log.Fatal(banner)
}

func parseOptions() config {
	if len(os.Args) < 3 {
		usage()
	}

	config := config{
		Username: os.Args[1],
		Password: os.Args[2],
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
	config := parseOptions()
	scraper := newScraper(config)

	if _, err := os.Stat("lessons.txt"); os.IsNotExist(err) {
		// lessons.txt does not exist

		// Find all tags from /index
		tags, err := scraper.GetAvailableTags()
		tags = removeDuplicateTags(tags)
		log.Printf("Found %v tags", len(tags))
		if err != nil {
			log.Fatal(err)
		}

		// Find all lessons from /tags pages
		lessonUrls, err := scraper.GetLessonUrlsFromTags(tags)
		log.Printf("Found %v lesson urls", len(lessonUrls))
		if err != nil {
			log.Fatal(err)
		}

		err = writeUrlsToFile(lessonUrls, "lessons.txt")
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Wrote %v lesson urls to lessons.txt", len(lessonUrls))
	}

	lessonUrls, err := readUrlsFromFile("lessons.txt")
	if err != nil {
		log.Fatal(err)
	}

	// First login
	log.Println("Logging in")
	err = scraper.Login()
	if err != nil {
		log.Fatal(err)
	}

	// // Loop all lessons and download them
	for _, url := range lessonUrls {
		fmt.Println(url)
		err := scraper.DownloadLessonFromURL(url)
		if err != nil {
			log.Printf("skiping %s: %v", url, err)
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
	BaseURL   string
	Client    http.Client
	Username  string
	Password  string
	Directory string
}

type lesson struct {
	ID   int
	Name string
	URL  string
	Type string
}

type tag struct {
	Name string
	URL  string
}

type serie struct {
	ID   string
	Name string
	URL  string
}

// Determine what the proper filename for a lesson should be
func (l *lesson) GetFilename(contentType string) (string, error) {
	basename := ""
	pieces := strings.Split(l.URL, "/")

	if l.Type == "episode" {
		basename = pieces[len(pieces)-3]
	} else {
		basename = pieces[len(pieces)-1]
	}

	pieces = strings.Split(contentType, "/")
	extension := pieces[len(pieces)-1]

	return fmt.Sprintf("%v-%v.%v", strconv.Itoa(l.ID), basename, extension), nil
}

func newScraper(config config) scraper {
	s := scraper{}
	s.BaseURL = "https://laracasts.com"

	s.Username = config.Username
	s.Password = config.Password
	s.Directory = config.Directory

	jar, _ := cookiejar.New(nil)
	s.Client = http.Client{
		Jar: jar,
	}
	return s
}

// Find all tags on /index
// To get the tags you don't have to be logged in
func (s *scraper) GetAvailableTags() ([]tag, error) {
	tags := []tag{}

	url := s.BaseURL + "/index"
	resp, err := s.Client.Get(url)
	if err != nil {
		return tags, err
	}

	doc, err := goquery.NewDocumentFromResponse(resp)

	// Find all links to tags
	links := doc.Find("#index li > a")
	links.Each(func(i int, q *goquery.Selection) {
		href, _ := q.Attr("href")
		name, _ := q.Html()
		name = strings.TrimSpace(name)

		tag := tag{
			Name: name,
			URL:  href,
		}

		tags = append(tags, tag)
	})

	tags = removeDuplicateTags(tags)
	return tags, nil
}

func (s *scraper) GetLessonUrlsFromTags(tags []tag) ([]string, error) {
	var mutex sync.Mutex
	lessonUrls := []string{}
	wg := sync.WaitGroup{}
	wg.Add(len(tags))

	for i := range tags {
		tag := tags[i]
		func() {
			urls, err := s.GetLessonUrlsFromTag(tag)
			if err != nil {
				log.Fatal(err)
			}

			mutex.Lock()
			lessonUrls = append(lessonUrls, urls...)
			mutex.Unlock()
			wg.Done()
		}()
	}

	wg.Wait()
	lessonUrls = removeDuplicateStrings(lessonUrls)
	return lessonUrls, nil
}

func (s *scraper) GetLessonUrlsFromTag(tag tag) ([]string, error) {
	lessonUrls := []string{}

	url := tag.URL
	resp, err := s.Client.Get(url)
	if err != nil {
		return lessonUrls, err
	}

	if resp.StatusCode != 200 {
		return lessonUrls, fmt.Errorf("%s returned wrong status code: %v, expected %v",
			url,
			resp.StatusCode,
			200,
		)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)

	links := doc.Find(".Lesson-List li a")
	links.Each(func(i int, q *goquery.Selection) {
		href, _ := q.Attr("href")
		href = s.BaseURL + href
		name, _ := q.Html()
		name = strings.TrimSpace(name)

		// https://laracasts.com/lessons/faster-workflow-with-generators
		matched, _ := regexp.MatchString(`\/lessons/.*`, href)
		if matched {
			lessonUrls = append(lessonUrls, href)
			return
		}

		// https://laracasts.com/series/es6-cliffsnotes/episodes/16
		matched, _ = regexp.MatchString(`\/series\/.*\/episodes\/`, href)
		if matched {
			lessonUrls = append(lessonUrls, href)
			return
		}

		// https://laracasts.com/series/es6-cliffsnotes
		matched, _ = regexp.MatchString(`\/series\/.*`, href)
		if matched {
			urls, err := s.GetLessonURLsFromSeriesURL(href)
			if err != nil {
				return
			}

			lessonUrls = append(lessonUrls, urls...)
			return
		}
	})

	return lessonUrls, nil
}

func (s *scraper) GetLessonURLsFromSeriesURL(url string) ([]string, error) {
	lessonUrls := []string{}

	resp, err := s.Client.Get(url)
	if err != nil {
		return lessonUrls, err
	}

	if resp.StatusCode != 200 {
		return lessonUrls, fmt.Errorf("%s returned wrong status code: %v, expected %v",
			url,
			resp.StatusCode,
			200,
		)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)

	links := doc.Find(".Lesson-List__title a")
	links.Each(func(i int, q *goquery.Selection) {
		href, _ := q.Attr("href")
		href = s.BaseURL + href

		lessonUrls = append(lessonUrls, href)
		return
	})

	return lessonUrls, nil
}


// Find all lesson on /all
// To get the lessonID you have to be logged in
func (s *scraper) GetAvailableLessons() ([]lesson, error) {
	episodes := []lesson{}

	url := s.BaseURL + "/index"
	resp, err := s.Client.Get(url)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromResponse(resp)

	// Find all links to lessons
	links := doc.Find(".container a.js-lesson-title")
	links.Each(func(i int, q *goquery.Selection) {
		href, _ := q.Attr("href")
		name, _ := q.Html()

		// Find the lessonID
		p := q.ParentsFiltered("li")
		input := p.Find("[name='lesson-id']")
		str, _ := input.Attr("value")
		lessonID, _ := strconv.Atoi(str)
		typ, _ := p.Find("[name='type']").Attr("value")
		typ = strings.ToLower(typ)
		typ = strings.Replace(typ, "laracasts\\", "", -1)

		lesson := lesson{}
		lesson.ID = lessonID
		lesson.URL = href
		lesson.Name = name
		lesson.Type = typ

		episodes = append(episodes, lesson)
	})

	return episodes, nil
}

// Login to laracasts
func (s *scraper) Login() error {
	token, err := s.GetToken()
	if err != nil {
		return err
	}

	u := s.BaseURL + "/sessions"
	resp, err := s.Client.PostForm(u,
		url.Values{
			"email":    {s.Username},
			"password": {s.Password},
			"_token":   {token},
		},
	)
	defer resp.Body.Close()

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s returned wrong status code: %v, expected %v",
			u,
			resp.StatusCode,
			200,
		)
	}

	return nil
}

// Login to laracasts
func (s *scraper) GetToken() (string, error) {
	url := s.BaseURL + "/"
	resp, err := s.Client.Get(url)
	defer resp.Body.Close()

	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s returned wrong status code: %v, expected %v",
			url,
			resp.StatusCode,
			200,
		)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	input := doc.Find("login-button[token]")
	token, _ := input.Attr("token")
	if err != nil {
		return "", fmt.Errorf("Can't find input with name _token")
	}

	return token, nil
}

func (s *scraper) DownloadLessonFromURL(url string) error {
	resp, err := s.Client.Get(url)
	defer resp.Body.Close()

	if err != nil {
		log.Println("Error while downloading", url, "-", err)
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s returned wrong status code: %v, expected %v",
			url,
			resp.StatusCode,
			200,
		)
	}

	html, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}

	doc := goquery.NewDocumentFromNode(html)
	serie := s.GetSerieFromDoc(doc)

	downloadURL := s.GetDownloadFromDoc(doc)
	return s.DownloadLesson(serie, downloadURL)
}

// Download a specific lesson and put it in a directory
func (s *scraper) DownloadLesson(serie *serie, url string) error {
	resp, err := s.Client.Get(url)
	if err != nil {
		log.Println("Error while downloading", url, "-", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s returned wrong status code: %v, expected %v",
			url,
			resp.StatusCode,
			200,
		)
	}

	header := resp.Header["Content-Disposition"][0]
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return err
	}
	filename := params["filename"] // set to "foo.png"

	dir := s.Directory
	if serie != nil {
		dir = filepath.Join(dir, serie.ID)
	}

	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, filename)

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
		if fileInfo.Size() == resp.ContentLength {
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

func removeDuplicateTags(tags []tag) []tag {
	result := []tag{}
	seen := map[string]tag{}
	for _, tag := range tags {
		if _, ok := seen[tag.URL]; !ok {
			result = append(result, tag)
			seen[tag.URL] = tag
		}
	}
	return result
}

func removeDuplicateStrings(strings []string) []string {
	result := []string{}
	seen := map[string]string{}
	for _, str := range strings {
		if _, ok := seen[str]; !ok {
			result = append(result, str)
			seen[str] = str
		}
	}
	return result
}

func writeUrlsToFile(urls []string, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, url := range urls {
		_, err := fmt.Fprintf(f, "%s\n", url)
		if err != nil {
			return err
		}
	}

	w.Flush()
	return nil
}

func readUrlsFromFile(filename string) ([]string, error) {
	urls := []string{}

	f, err := os.Open(filename)
	if err != nil {
		return urls, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}

		if err != nil {
			return urls, err
		}

		urls = append(urls, string(line))
	}

	return urls, nil

}

func (s *scraper) GetSerieFromDoc(doc *goquery.Document) *serie {
	h2 := doc.Find(".Video__body h2 a")
	if h2.Length() == 0 {
		return nil
	}

	seriesURL, _ := h2.Attr("href")
	seriesURL = s.BaseURL + seriesURL
	seriesName, _ := h2.Html()
	seriesName = strings.TrimSpace(seriesName)
	seriesName = strings.Trim(seriesName, ":")

	re, err := regexp.Compile(`\/series\/(.*)(\/|$)`)
	if err != nil {
		return nil
	}

	seriesID := ""
	matches := re.FindStringSubmatch(seriesURL)
	if len(matches) > 1 {
		seriesID = matches[1]
	}

	serie := &serie{
		ID:   seriesID,
		Name: seriesName,
		URL:  seriesURL,
	}

	return serie
}

func (s *scraper) GetDownloadFromDoc(doc *goquery.Document) string {
	a := doc.Find("a[href*='/downloads']")
	if a.Length() == 0 {
		return ""
	}

	url, _ := a.Attr("href")
	url = s.BaseURL + url
	return url
}
