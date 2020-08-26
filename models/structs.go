package models

import "time"

// Video stores the important fields in the video
type Video struct {
	TitleURL string    `json:"titleUrl"`
	Time     time.Time `json:"time"`
}

// Data holds a field of the response of the Youtube API
type Data struct {
	Items []Item `json:"items"`
}

// Item holds the content details of the video
type Item struct {
	ContentDetails ContentDetails `json:"contentDetails"`
}

// ContentDetails holds the video duration
type ContentDetails struct {
	Duration string `json:"duration"`
}

// RequestBasicInfo stores the basic information about the query
type RequestBasicInfo struct {
	Population         int
	SampleSize         int
	MissingLinks       int
	MissingLinksSample int
}

// RequestAdvancedInfo stores the processed information about the query
type RequestAdvancedInfo struct {
	YearInfo             map[int]int
	TotalDurationSeconds int64
	TotalDurationSample  int64
	AvgDuration          float64
}

// Response stores the response that will receive the frontend
type Response struct {
	RequestBasicInfo    RequestBasicInfo
	RequestAdvancedInfo RequestAdvancedInfo
}
