package main

import (
	"blurlconvert/blurldecrypt"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <input.blurl|input.json> <output>")
		return
	}

	if !strings.HasSuffix(os.Args[1], ".blurl") && !strings.HasSuffix(os.Args[1], ".json") {
		fmt.Println("The Input File must be a .blurl or .json file")
		return
	}

	var blurl BLURL

	if strings.HasSuffix(os.Args[1], ".blurl") {
		err := parseBLURL(&blurl, os.Args[1])
		if err != nil {
			fmt.Printf("Error parsing BLURL file: %v\n", err)
			return
		}
	} else {
		err := parseBLURLFromJSON(&blurl, os.Args[1])
		if err != nil {
			fmt.Printf("Error parsing JSON file: %v\n", err)
			return
		}
	}

	var mediaurl string
	if len(blurl.Playlists) == 1 {
		mediaurl = blurl.Playlists[0].URL
	} else {
		mediaurl = GetMediaURL(&blurl)
	}

	if mediaurl == "" {
		fmt.Println("No valid media URL selected")
		return
	}

	var key []byte

	if len(blurl.Ev) > 0 {
		decodedEV, err := base64.StdEncoding.DecodeString(blurl.Ev)
		if err != nil {
			fmt.Println("Error decoding base64:", err)
			return
		}

		parsedev, err := blurldecrypt.ParseEV(decodedEV)
		if err != nil {
			fmt.Println("Error parsing EV:", err)
			return
		}

		key = blurldecrypt.GetEncryptionKey("keys.bin", parsedev.Nonce, parsedev.Key[:])
		if key == nil {
			fmt.Println("Failed to get encryption key")
			return
		}

		fmt.Printf("Decryption Key: %02x\n", key)
	}

	mediaurl, err := RemoveDuplicateUUIDPath(mediaurl)
	if err != nil {
		fmt.Printf("Error processing URL: %v\n", err)
		return
	}

	mpddata, err := GetPlaylistMetadataByID(mediaurl)
	if err != nil {
		fmt.Println("Error getting playlist metadata:", err)
		return
	}

	trackduration := GetPlaylistDuration(mpddata)
	if trackduration <= 0 {
		fmt.Println("Track Duration is 0 or invalid, exiting!")
		return
	}

	if len(mpddata.Period.AdaptationSet) == 0 {
		fmt.Println("No AdaptationSet found in MPD, exiting!")
		return
	}

	numberOfSegments := 0.0

	firstSet := mpddata.Period.AdaptationSet[0]
	if len(firstSet.Representation) == 0 {
		fmt.Println("No Representation found in MPD, exiting!")
		return
	}

	bestRepIndex := 0
	bestBw := int64(-1)
	for i, r := range firstSet.Representation {
		bw, _ := strconv.ParseInt(r.Bandwidth, 10, 64)
		if bw > bestBw {
			bestBw = bw
			bestRepIndex = i
		}
	}

	segmentDurationStr := firstSet.Representation[bestRepIndex].SegmentTemplate.Duration
	segmentTimescaleStr := firstSet.Representation[bestRepIndex].SegmentTemplate.Timescale

	if segmentDurationStr == "" {
		segmentDurationStr = firstSet.SegmentTemplate.Duration
	}
	if segmentTimescaleStr == "" {
		segmentTimescaleStr = firstSet.SegmentTemplate.Timescale
	}

	if segmentDurationStr != "" && segmentTimescaleStr != "" {
		segmentDuration, err := strconv.ParseInt(segmentDurationStr, 10, 64)
		if err != nil {
			fmt.Println("Error parsing segment duration:", err)
			return
		}

		timescale, err := strconv.ParseInt(segmentTimescaleStr, 10, 64)
		if err != nil {
			fmt.Println("Error parsing timescale:", err)
			return
		}

		numberOfSegments = math.Ceil(trackduration / (float64(segmentDuration) / float64(timescale)))
	} else {
		numberOfSegments = 1
	}

	if numberOfSegments <= 0 {
		fmt.Println("Invalid number of track segments! exiting.")
		return
	}

	fmt.Printf("===================================================================================\n")
	fmt.Printf("Media Codec: %s\n", mpddata.Period.AdaptationSet[0].Representation[bestRepIndex].Codecs)
	fmt.Printf("Sample Rate: %skHz\n", mpddata.Period.AdaptationSet[0].Representation[bestRepIndex].AudioSamplingRate)
	fmt.Printf("===================================================================================\n")

	if !isDirExists("downloads") {
		err := os.Mkdir("downloads", 0755)
		if err != nil {
			fmt.Printf("Error creating downloads directory: %v\n", err)
			return
		}
	}

	for _, adaptation := range mpddata.Period.AdaptationSet {
		if len(adaptation.Representation) == 0 {
			fmt.Printf("Error Downloading Track: no representations\n")
			continue
		}

		repIndex := 0
		repBw := int64(-1)
		for i, r := range adaptation.Representation {
			bw, _ := strconv.ParseInt(r.Bandwidth, 10, 64)
			if bw > repBw {
				repBw = bw
				repIndex = i
			}
		}

		contentType := strings.TrimSpace(adaptation.ContentType)
		if contentType == "" {
			m := strings.ToLower(strings.TrimSpace(adaptation.Representation[repIndex].MimeType))
			if strings.Contains(m, "audio/") {
				contentType = "audio"
			} else if strings.Contains(m, "video/") {
				contentType = "video"
			} else {
				contentType = "audio"
			}
		}

		fmt.Printf("Processing %s track...\n", contentType)

		initTpl := adaptation.Representation[repIndex].SegmentTemplate.Initialization
		mediaTpl := adaptation.Representation[repIndex].SegmentTemplate.Media
		startNumStr := adaptation.Representation[repIndex].SegmentTemplate.StartNumber

		if initTpl == "" {
			initTpl = adaptation.SegmentTemplate.Initialization
		}
		if mediaTpl == "" {
			mediaTpl = adaptation.SegmentTemplate.Media
		}
		if startNumStr == "" {
			startNumStr = adaptation.SegmentTemplate.StartNumber
		}

		startNumber := 1
		if startNumStr != "" {
			v, e := strconv.Atoi(startNumStr)
			if e == nil && v > 0 {
				startNumber = v
			}
		}

		initFile := ""
		if initTpl != "" {
			initFile = strings.ReplaceAll(initTpl, "$RepresentationID$", adaptation.Representation[repIndex].ID)
		} else {
			initFile = strings.TrimSpace(adaptation.Representation[repIndex].BaseURL)
		}

		fullFileURL := ""
		initRange := ""
		indexRange := ""

		if mediaTpl == "" {
			baseName := strings.TrimSpace(adaptation.Representation[repIndex].BaseURL)
			if baseName != "" {
				fullFileURL = getBaseURL(mediaurl) + baseName
				initRange = adaptation.Representation[repIndex].SegmentBase.Initialization.Range
				indexRange = adaptation.Representation[repIndex].SegmentBase.IndexRange
			}
		}

		err := HandleDownloadTrack(
			contentType,
			fmt.Sprintf("master_%s", contentType),
			numberOfSegments,
			getBaseURL(mediaurl),
			initFile,
			adaptation.Representation[repIndex].ID,
			hex.EncodeToString(key),
			mediaTpl,
			startNumber,
			fullFileURL,
			initRange,
			indexRange,
		)

		if err != nil {
			fmt.Printf("Error Downloading %s Track: %v\n", contentType, err)
			continue
		}
	}

	if len(mpddata.Period.AdaptationSet) == 2 {
		videoFile := "master_video.mp4"
		audioFile := "master_audio.mp4"

		if _, err := os.Stat(videoFile); err == nil {
			if _, err := os.Stat(audioFile); err == nil {
				fmt.Println("Merging audio and video tracks...")
				Merge(videoFile, audioFile, EncodeToBase62(mpddata.Period.AdaptationSet[0].ContentProtection[0].DefaultKID)[:8])
			}
		}
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Cleaning up temporary files...")
	os.RemoveAll("./downloads")

	fmt.Println("Process completed successfully!")
}
