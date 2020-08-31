package utils

import (
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/tomsarry/woyt_backend/models"
)

// shuffleUrls is the Fisher-Yates method to shuffle array of strings
func shuffleUrls(urls []string) {
	N := len(urls)
	for i := 0; i < N; i++ {
		// choose index uniformly in [i, N-1]
		r := i + rand.Intn(N-i)
		urls[r], urls[i] = urls[i], urls[r]
	}
}

// getUrlsShuffled retrieves every url from an array of videos objects and shuffle them
// also returning the number of missing links
func getUrlsShuffled(videos []models.Video) ([]string, int) {
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

// parseInt64 returns the int64 representation of a given string
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

// parseDuration converts an ISO8601 string to an int
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

// UpdateDurationSample updates the total time of each video
func UpdateDurationSample(durations models.Data) (int64, int) {
	var totalDuration int64 = 0
	var outOfRange int = 0

	for _, duration := range durations.Items {
		computedDuration, comuptedOutOfRange := parseDuration(duration.ContentDetails.Duration)

		totalDuration += computedDuration
		outOfRange += comuptedOutOfRange
	}

	return totalDuration, outOfRange
}

// GetTotalDuration computes the total duration of the videos, with the estimation of the sample
func GetTotalDuration(sampleDuration int64, sampleSize int, totalSize int, missingLinksSample int) int64 {
	// compute an average duration for a video
	avgTimeSample := sampleDuration / (int64(sampleSize) - int64(missingLinksSample))

	return avgTimeSample * int64(totalSize)
}

// GetSampleSize will compute the amount of videos that should be requested to the
// YT API to have an accurate estimation of the total, given the total amount of videos
func GetSampleSize(numVideo int) int {

	if numVideo < 400 {
		return numVideo
	}

	// this value controls the accuracy, decrease it to have more videos checked, and
	// a more precise estimation
	var marginError float64 = 0.03

	sampleSize := numVideo / (1 + int(float64(numVideo)*marginError*marginError))
	return int(sampleSize)
}

// GetIDSample returns a sample of videos
func GetIDSample(videos []models.Video) ([]string, int) {

	sampleSize := GetSampleSize(len(videos))

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

// GetUrlsAPI creates the URLs to make the API requests
func GetUrlsAPI(ids []string) []string {

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
