package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tomsarry/woyt_backend/models"
	"github.com/tomsarry/woyt_backend/utils"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func inSlice(slc []models.YearInfo, year int) (bool, int) {
	for i := 0; i < len(slc); i++ {
		if slc[i].Year == year {
			return true, i
		}
	}
	return false, -1
}

// UploadHandler handles the processing and the response of the user request
func UploadHandler(c *gin.Context) {

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
	var videos []models.Video

	err = json.Unmarshal(byteValue, &videos)

	if err != nil {
		fmt.Println(err)
	}

	sampleSize := utils.GetSampleSize(len(videos))
	population := len(videos)

	var yearValues []models.YearInfo

	for i := 0; i < population; i++ {
		// get when the video has been watched (year only)
		yearWatched := videos[i].Time.Year()

		found, index := inSlice(yearValues, yearWatched)
		if found {
			yearValues[index].Value++
		} else {
			yearValues = append(yearValues, models.YearInfo{Year: yearWatched, Value: 1})
		}
	}

	ids, missingLinks := utils.GetIDSample(videos)

	urlsAPI := utils.GetUrlsAPI(ids)

	// if didn't find any video id, send an error to frontend, stop the program
	if len(urlsAPI) == 0 {
		fmt.Println("Error with file, check that it is your watched history and not searched history")
		c.String(http.StatusBadRequest, fmt.Sprintf("Error with file, check that it is your watched history and not searched history"))
		return
	}

	// any missing links in the sampling ?
	missingLinksSample := sampleSize - len(ids)

	var listData models.Data
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

			comuptedDurationSample, computedOutOfRange := utils.UpdateDurationSample(listData)

			outOfRange += computedOutOfRange
			totalDurationSample += comuptedDurationSample

		}(i, url)
	}

	wg.Wait()

	// compute the total duration for all the videos from the total given by the sample
	// adding the missingLinks with the outOfRange length that returned a duration of 0
	totalDuration = utils.GetTotalDuration(totalDurationSample, sampleSize, population, missingLinksSample+outOfRange)

	avgDuration := float64(totalDuration / int64(population))

	// divide the respones in two structs
	basicInfo := models.RequestBasicInfo{Population: population, SampleSize: sampleSize, MissingLinks: missingLinks, MissingLinksSample: missingLinksSample + outOfRange}
	advancedInfo := models.RequestAdvancedInfo{YearInfos: yearValues, TotalDurationSeconds: totalDuration, TotalDurationSample: totalDurationSample, AvgDuration: avgDuration}

	// creating a struct holding the response
	res := models.Response{RequestBasicInfo: basicInfo, RequestAdvancedInfo: advancedInfo}

	fmt.Println("total duration is :", totalDuration)
	c.JSON(200, res)

	// don't forget to close the file
	defer jsonFile.Close()

}
