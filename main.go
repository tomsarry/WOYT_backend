package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// Video stores the important fields in the video
type Video struct {
	TitleURL string    `json:"titleUrl"`
	Time     time.Time `json:"time"`
}

// RequestInfo stores the information of the request for the YT API
type RequestInfo struct {
	Population           int
	SampleSize           int
	YearInfo             map[int]int
	MissingLinks         int
	MissingLinksSample   int
	TotalDurationSeconds int64
	TotalDurationSample  int64
	AvgDuration          float64
}

// Data holds the response of the Youtube API
type Data struct {
	Items []Item `json:"items"`
}

// Item holds the information of the video
type Item struct {
	ContentDetails ContentDetails `json:"contentDetails"`
}

// ContentDetails holds the video duration
type ContentDetails struct {
	Duration string `json:"duration"`
}

// retrieve the env variables first
func init() {
	godotenv.Load()
	rand.Seed(time.Now().UTC().UnixNano())
}

func computeSampleSize(videos []Video) int {
	populationSize := len(videos)

	if populationSize < 300 {
		fmt.Println("Population is small, testing every video.")
		return int(populationSize)
	}
	// wanted accuracy of 98%
	var marginError float64 = 0.03

	sampleSize := populationSize / (1 + int(float64(populationSize)*marginError*marginError))
	return int(sampleSize)
}

//  Fisher-Yates method to shuffle array of strings
func shuffleUrls(urls []string) {
	N := len(urls)
	for i := 0; i < N; i++ {
		// choose index uniformly in [i, N-1]
		r := i + rand.Intn(N-i)
		urls[r], urls[i] = urls[i], urls[r]
	}
}

// get every url and shuffle them, reuturns the number of missing links
func getUrlsShuffled(videos []Video) ([]string, int) {
	var urls []string
	missingLinks := 0

	// if missing link, skip it
	for _, video := range videos {
		if video.TitleURL == "" {
			missingLinks++
			continue
		} else {
			urls = append(urls, video.TitleURL)
		}

	}
	shuffleUrls(urls)

	return urls, missingLinks
}

func getIDSample(sampleSize int, videos []Video) ([]string, int) {

	urls, missingLinks := getUrlsShuffled(videos)
	var ids []string
	ctn := 0

	// want enough ids to make an accurate sample, but not more than the total population
	for cur := 0; ctn < sampleSize && cur < len(urls); cur++ {

		u, err := url.Parse(urls[cur])

		if err != nil {
			continue
		}

		m, err := url.ParseQuery(u.RawQuery)

		if len(m["v"]) == 0 {
			continue
		}

		if err != nil {
			continue
		}
		ids = append(ids, m["v"][0])
		ctn++
	}

	return ids, missingLinks
}

// creates the URLs to make the API requests
func getUrlsAPI(ids []string) []string {

	// if didn't find any ids, send error
	if len(ids) == 0 {
		var empty []string
		return empty
	}

	numRequest := (len(ids) / 50) + 1
	var apiUrls []string

	for i := 0; i < numRequest; i++ {
		var size int
		// if not 50 more ids, then take the rest
		if (i+1)*50 > len(ids) {
			size = len(ids)
		} else {
			size = (i + 1) * 50
		}
		listIDs := strings.Join(ids[i*50:size], ",")
		apiURL := "https://www.googleapis.com/youtube/v3/videos?part=contentDetails&id=" + listIDs + "&key=" + os.Getenv("API_KEY")
		apiUrls = append(apiUrls, apiURL)
	}
	return apiUrls
}

// converts an ISO8601 string to an int
func parseDuration(str string) (int64, int) {
	durationRegex := regexp.MustCompile(`P(?P<years>\d+Y)?(?P<months>\d+M)?(?P<days>\d+D)?T?(?P<hours>\d+H)?(?P<minutes>\d+M)?(?P<seconds>\d+S)?`)
	matches := durationRegex.FindStringSubmatch(str)

	years := parseInt64(matches[1])
	months := parseInt64(matches[2])
	days := parseInt64(matches[3])
	hours := parseInt64(matches[4])
	minutes := parseInt64(matches[5])
	seconds := parseInt64(matches[6])

	videoDuration := years*24*365*60*60 + months*30*24*60*60 + days*24*60*60 + hours*60*60 + minutes*60 + seconds

	// if the video is longer that a day (stream), then don't return a value
	if days > 0 {
		return int64(0), 1
	}

	return videoDuration, 0
}

func parseInt64(value string) int64 {
	if len(value) == 0 {
		return 0
	}
	parsed, err := strconv.Atoi(value[:len(value)-1])
	if err != nil {
		return 0
	}
	return int64(parsed)
}

// updates the total time of each video
func updateDurationSample(durations Data) (int64, int) {
	var totalDuration int64 = 0
	var outOfRange int = 0

	for _, duration := range durations.Items {
		computedDuration, comuptedOutOfRange := parseDuration(duration.ContentDetails.Duration)

		totalDuration += computedDuration
		outOfRange += comuptedOutOfRange
	}

	return totalDuration, outOfRange
}

// computes the total duration of the videos, with the estimation of the sample
func getTotalDuration(sampleDuration int64, sampleSize int, totalSize int, missingLinksSample int) int64 {
	// compute an average duration for a video
	avgTimeSample := sampleDuration / (int64(sampleSize) - int64(missingLinksSample))

	return avgTimeSample * int64(totalSize)
}

func main() {

	r := gin.Default()
	r.MaxMultipartMemory = 8 << 20
	r.Static("/", "./public")
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{os.Getenv("WEBSITE")},
		AllowMethods: []string{"GET", "PUT", "POST"},
	}))

	r.POST("/upload", func(c *gin.Context) {

		file, err := c.FormFile("file")

		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get form err %s", err.Error()))
			return
		}

		filename := filepath.Base(file.Filename)
		if err := c.SaveUploadedFile(file, filename); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("uploaded file err: %s", err.Error()))
			return
		}

		// opening the file received as input
		jsonFile, err := os.Open(filename)

		// handling any errors
		if err != nil {
			fmt.Println(err)
		}

		byteValue, err := ioutil.ReadAll(jsonFile)

		if err != nil {
			fmt.Println(err)
		}

		// initialize the array of videos
		var videos []Video

		err = json.Unmarshal(byteValue, &videos)

		if err != nil {
			fmt.Println(err)
		}

		sampleSize := computeSampleSize(videos)
		population := len(videos)

		yearValues := make(map[int]int)

		for i := 0; i < population; i++ {
			// get when the video has been watched (year only)
			yearWatched := videos[i].Time.Year()

			_, ok := yearValues[yearWatched]

			// check to see if the map already has values for this year
			if !ok {
				yearValues[yearWatched] = 1
			} else {
				yearValues[yearWatched]++
			}

		}

		ids, missingLinks := getIDSample(sampleSize, videos)

		urlsAPI := getUrlsAPI(ids)

		fmt.Printf("making %d  API requests, checking %d videos", len(urlsAPI), sampleSize)

		// if didn't find any video id, send an error to frontend, stop the program
		if len(urlsAPI) == 0 {
			fmt.Println("Error with file, check that it is your watched history and not searched history")
			c.String(http.StatusBadRequest, fmt.Sprintf("Error with file, check that it is your watched history and not searched history"))
			return
		}

		// any missing links in the sampling ?
		missingLinksSample := sampleSize - len(ids)

		fmt.Println(missingLinks, missingLinksSample)

		var listData Data
		var totalDurationSample int64 = 0
		var totalDuration int64 = 0
		var outOfRange int = 0

		// initialize multi-threading
		var wg sync.WaitGroup
		wg.Add(len(urlsAPI))

		for i, url := range urlsAPI {

			go func(i int, url string) {
				defer wg.Done()
				resp, err := http.Get(url)

				if err != nil {
					fmt.Println(err)
					panic(err.Error())
				}

				data, _ := ioutil.ReadAll(resp.Body)
				err = json.Unmarshal(data, &listData)

				if err != nil {
					fmt.Println("error :", err)
					panic(err.Error())
				}

				// see each video duration in ISO8601 before parsing
				// fmt.Println(listData)

				comuptedDurationSample, computedOutOfRange := updateDurationSample(listData)

				outOfRange += computedOutOfRange
				totalDurationSample += comuptedDurationSample

				//testing
				// fmt.Println("duration of sample is : ", totalDurationSample)
				// fmt.Println("number of videos out of range : ", outOfRange)

			}(i, url)
		}

		wg.Wait()

		// compute the total duration for all the videos from the total given by the sample
		// adding the missingLinks with the outOfRange length that returned a duration of 0
		totalDuration = getTotalDuration(totalDurationSample, sampleSize, population, missingLinksSample+outOfRange)

		avgDuration := float64(totalDuration / int64(population))
		message := RequestInfo{population, sampleSize, yearValues, missingLinks, missingLinksSample + outOfRange, totalDuration, totalDurationSample, avgDuration}

		fmt.Println("total duration is :", totalDuration)
		c.JSON(200, message)

		// don't forget to close the file
		defer jsonFile.Close()
	})

	r.Run()
}
