package main

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PlaylistMetadata struct {
	Playlist     string `json:"playlist"`
	PlaylistType string `json:"playlistType"`
	Metadata     struct {
		AssetID         string   `json:"assetId"`
		BaseUrls        []string `json:"baseUrls"`
		SupportsCaching bool     `json:"supportsCaching"`
		Ucp             string   `json:"ucp"`
		Version         string   `json:"version"`
	} `json:"metadata"`
}

type MPD struct {
	XMLName                   xml.Name `xml:"MPD"`
	Text                      string   `xml:",chardata"`
	Xmlns                     string   `xml:"xmlns,attr"`
	Xsi                       string   `xml:"xsi,attr"`
	Xlink                     string   `xml:"xlink,attr"`
	SchemaLocation            string   `xml:"schemaLocation,attr"`
	Clearkey                  string   `xml:"clearkey,attr"`
	Cenc                      string   `xml:"cenc,attr"`
	Profiles                  string   `xml:"profiles,attr"`
	Type                      string   `xml:"type,attr"`
	MediaPresentationDuration string   `xml:"mediaPresentationDuration,attr"`
	MaxSegmentDuration        string   `xml:"maxSegmentDuration,attr"`
	MinBufferTime             string   `xml:"minBufferTime,attr"`
	BaseURL                   string   `xml:"BaseURL"`
	ProgramInformation        string   `xml:"ProgramInformation"`
	Period                    struct {
		Text          string `xml:",chardata"`
		ID            string `xml:"id,attr"`
		Start         string `xml:"start,attr"`
		AdaptationSet []struct {
			Text               string `xml:",chardata"`
			ID                 string `xml:"id,attr"`
			ContentType        string `xml:"contentType,attr"`
			StartWithSAP       string `xml:"startWithSAP,attr"`
			SegmentAlignment   string `xml:"segmentAlignment,attr"`
			BitstreamSwitching string `xml:"bitstreamSwitching,attr"`
			SegmentTemplate    struct {
				Text           string `xml:",chardata"`
				Duration       string `xml:"duration,attr"`
				Timescale      string `xml:"timescale,attr"`
				Initialization string `xml:"initialization,attr"`
				Media          string `xml:"media,attr"`
				StartNumber    string `xml:"startNumber,attr"`
			} `xml:"SegmentTemplate"`
			Representation []struct {
				Text              string `xml:",chardata"`
				ID                string `xml:"id,attr"`
				AudioSamplingRate string `xml:"audioSamplingRate,attr"`
				Bandwidth         string `xml:"bandwidth,attr"`
				MimeType          string `xml:"mimeType,attr"`
				Codecs            string `xml:"codecs,attr"`
				BaseURL           string `xml:"BaseURL"`
				SegmentBase       struct {
					Text            string `xml:",chardata"`
					IndexRange      string `xml:"indexRange,attr"`
					IndexRangeExact string `xml:"indexRangeExact,attr"`
					Initialization  struct {
						Text  string `xml:",chardata"`
						Range string `xml:"range,attr"`
					} `xml:"Initialization"`
				} `xml:"SegmentBase"`
				SegmentTemplate struct {
					Text           string `xml:",chardata"`
					Duration       string `xml:"duration,attr"`
					Timescale      string `xml:"timescale,attr"`
					Initialization string `xml:"initialization,attr"`
					Media          string `xml:"media,attr"`
					StartNumber    string `xml:"startNumber,attr"`
				} `xml:"SegmentTemplate"`
				AudioChannelConfiguration struct {
					Text        string `xml:",chardata"`
					SchemeIdUri string `xml:"schemeIdUri,attr"`
					Value       string `xml:"value,attr"`
				} `xml:"AudioChannelConfiguration"`
			} `xml:"Representation"`
			ContentProtection []struct {
				Text        string `xml:",chardata"`
				SchemeIdUri string `xml:"schemeIdUri,attr"`
				Value       string `xml:"value,attr"`
				DefaultKID  string `xml:"default_KID,attr"`
				Laurl       struct {
					Text    string `xml:",chardata"`
					LicType string `xml:"Lic_type,attr"`
				} `xml:"Laurl"`
			} `xml:"ContentProtection"`
		} `xml:"AdaptationSet"`
	} `xml:"Period"`
}

func isDirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func isFileValid(filepath string) bool {
	info, err := os.Stat(filepath)
	if err != nil {
		return false
	}

	if info.Size() == 0 {
		os.Remove(filepath)
		return false
	}

	return true
}

const (
	maxRetries = 3
	retryDelay = 2 * time.Second
	timeout    = 30 * time.Second
)

const base62 = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func EncodeToBase62(s string) string {
	n := big.NewInt(0).SetBytes([]byte(s))
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := &big.Int{}

	var result string
	for n.Cmp(zero) != 0 {
		n.DivMod(n, base, mod)
		result = string(base62[mod.Int64()]) + result
	}
	return result
}

func getBaseURL(fullURL string) string {
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return ""
	}

	basePath := path.Dir(parsedURL.Path)
	if basePath == "/" {
		basePath = ""
	}

	return fmt.Sprintf("%s://%s%s/", parsedURL.Scheme, parsedURL.Host, basePath)
}

func RemoveDuplicateUUIDPath(inputURL string) (string, error) {
	u, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}

	segments := strings.Split(u.Path, "/")

	seenUUIDs := make(map[string]bool)
	filteredSegments := []string{}

	for _, segment := range segments {
		if _, seen := seenUUIDs[segment]; !seen && segment != "" {
			seenUUIDs[segment] = true
			filteredSegments = append(filteredSegments, segment)
		} else if segment == "" || !seenUUIDs[segment] {
			filteredSegments = append(filteredSegments, segment)
		}
	}

	u.Path = strings.Join(filteredSegments, "/")

	return u.String(), nil
}

func GetPlaylistMetadataByID(url string) (*MPD, error) {
	res, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var MPD_Data MPD

	err = xml.Unmarshal(body, &MPD_Data)
	if err != nil {
		return nil, err
	}

	return &MPD_Data, nil
}

func GetPlaylistDuration(mpddata *MPD) float64 {
	duration, err := time.ParseDuration(strings.ToLower(strings.TrimPrefix(mpddata.MediaPresentationDuration, "PT")))

	if err != nil {
		fmt.Printf("failed to parse time duration: %s\n", err.Error())
		return 0
	}

	return duration.Seconds()
}

func downloadWithRetry(url, filepath string) error {
	client := &http.Client{
		Timeout: timeout,
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.Get(url)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("failed after %d attempts: %v", maxRetries, err)
			}
			time.Sleep(retryDelay * time.Duration(attempt))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if attempt == maxRetries {
				return fmt.Errorf("bad status after %d attempts: %s", maxRetries, resp.Status)
			}
			time.Sleep(retryDelay * time.Duration(attempt))
			continue
		}

		tempFile := filepath + ".tmp"
		out, err := os.Create(tempFile)
		if err != nil {
			return err
		}

		_, err = io.Copy(out, resp.Body)
		out.Close()

		if err != nil {
			os.Remove(tempFile)
			if attempt == maxRetries {
				return err
			}
			time.Sleep(retryDelay * time.Duration(attempt))
			continue
		}

		if _, err := os.Stat(filepath); err == nil {
			os.Remove(filepath)
		}

		err = os.Rename(tempFile, filepath)
		if err != nil {
			err = copyFile(tempFile, filepath)
			if err != nil {
				os.Remove(tempFile)
				if attempt == maxRetries {
					return fmt.Errorf("failed to create final file: %v", err)
				}
				time.Sleep(retryDelay * time.Duration(attempt))
				continue
			}
			os.Remove(tempFile)
		}

		if !isFileValid(filepath) {
			os.Remove(filepath)
			if attempt == maxRetries {
				return fmt.Errorf("file integrity check failed after %d attempts", maxRetries)
			}
			time.Sleep(retryDelay * time.Duration(attempt))
			continue
		}

		return nil
	}

	return fmt.Errorf("unknown error downloading %s", url)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func parseByteRange(s string) (int64, int64, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range: %s", s)
	}
	a, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	b, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if a < 0 || b < a {
		return 0, 0, fmt.Errorf("invalid range bounds: %s", s)
	}
	return a, b, nil
}

func httpRangeGet(u string, start, end int64) ([]byte, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	client := &http.Client{Timeout: timeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusPartialContent && res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", res.Status)
	}

	return io.ReadAll(res.Body)
}

type sidxInfo struct {
	firstOffset        uint64
	referencedSizes    []uint32
	sidxBoxSize        uint64
	sidxBoxOffsetInBuf int64
}

func findAndParseSidx(buf []byte) (*sidxInfo, error) {
	i := 0
	for i+8 <= len(buf) {
		size := binary.BigEndian.Uint32(buf[i : i+4])
		typ := string(buf[i+4 : i+8])
		var boxSize uint64
		hdr := 8
		if size == 1 {
			if i+16 > len(buf) {
				return nil, fmt.Errorf("invalid large size box")
			}
			boxSize = binary.BigEndian.Uint64(buf[i+8 : i+16])
			hdr = 16
		} else {
			boxSize = uint64(size)
		}
		if boxSize < uint64(hdr) || i+int(boxSize) > len(buf) {
			return nil, fmt.Errorf("invalid box size")
		}
		if typ == "sidx" {
			box := buf[i : i+int(boxSize)]
			return parseSidxBox(box, int64(i), boxSize)
		}
		i += int(boxSize)
	}
	return nil, fmt.Errorf("sidx not found")
}

func parseSidxBox(box []byte, off int64, boxSize uint64) (*sidxInfo, error) {
	pos := 8
	if binary.BigEndian.Uint32(box[0:4]) == 1 {
		pos = 16
	}

	if pos+4 > len(box) {
		return nil, fmt.Errorf("invalid sidx")
	}

	version := box[pos]
	pos += 4

	if pos+8 > len(box) {
		return nil, fmt.Errorf("invalid sidx")
	}
	pos += 4
	timescale := binary.BigEndian.Uint32(box[pos : pos+4])
	_ = timescale
	pos += 4

	var earliest uint64
	var firstOffset uint64
	if version == 0 {
		if pos+8 > len(box) {
			return nil, fmt.Errorf("invalid sidx v0")
		}
		earliest = uint64(binary.BigEndian.Uint32(box[pos : pos+4]))
		_ = earliest
		pos += 4
		firstOffset = uint64(binary.BigEndian.Uint32(box[pos : pos+4]))
		pos += 4
	} else {
		if pos+16 > len(box) {
			return nil, fmt.Errorf("invalid sidx v1")
		}
		earliest = binary.BigEndian.Uint64(box[pos : pos+8])
		_ = earliest
		pos += 8
		firstOffset = binary.BigEndian.Uint64(box[pos : pos+8])
		pos += 8
	}

	if pos+4 > len(box) {
		return nil, fmt.Errorf("invalid sidx")
	}
	pos += 2
	refCount := binary.BigEndian.Uint16(box[pos : pos+2])
	pos += 2

	sizes := make([]uint32, 0, refCount)
	for j := 0; j < int(refCount); j++ {
		if pos+12 > len(box) {
			return nil, fmt.Errorf("invalid sidx refs")
		}
		ref := binary.BigEndian.Uint32(box[pos : pos+4])
		pos += 4
		size := ref & 0x7FFFFFFF
		sizes = append(sizes, size)
		pos += 8
	}

	return &sidxInfo{
		firstOffset:        firstOffset,
		referencedSizes:    sizes,
		sidxBoxSize:        boxSize,
		sidxBoxOffsetInBuf: off,
	}, nil
}

func countSegmentsFromSegmentBase(fullURL string, initRange string, indexRange string) (int, *sidxInfo, int64, error) {
	indexStart, indexEnd, err := parseByteRange(indexRange)
	if err != nil {
		return 0, nil, 0, err
	}
	idxBuf, err := httpRangeGet(fullURL, indexStart, indexEnd)
	if err != nil {
		return 0, nil, 0, err
	}

	sidx, err := findAndParseSidx(idxBuf)
	if err != nil {
		return 0, nil, 0, err
	}

	sidxStartInFile := indexStart + sidx.sidxBoxOffsetInBuf
	return len(sidx.referencedSizes), sidx, sidxStartInFile, nil
}

func HandleDownloadTrack(mediatype string, id string, numberofsegments float64, baseurl string, initmp4 string, adaptation string, key string, mediaTemplate string, startNumber int, fullFileURL string, initRange string, indexRange string) error {
	if !isDirExists("downloads") {
		err := os.Mkdir("downloads", 0755)
		if err != nil {
			return fmt.Errorf("error creating downloads directory: %v", err)
		}
	}

	if mediaTemplate == "" && initRange != "" && indexRange != "" && fullFileURL != "" {
		count, sidx, sidxStart, err := countSegmentsFromSegmentBase(fullFileURL, initRange, indexRange)
		if err != nil {
			return err
		}

		fmt.Printf("===================================================================================\n")
		fmt.Printf("Track Segments: %d\n", count)
		fmt.Printf("Media Type: %s\n", mediatype)
		fmt.Printf("===================================================================================\n")

		localName := fmt.Sprintf("%s.mp4", id)
		initPath := fmt.Sprintf("./downloads/%s", localName)

		os.Remove(initPath)

		initStart, initEnd, err := parseByteRange(initRange)
		if err != nil {
			return err
		}

		initBytes, err := httpRangeGet(fullFileURL, initStart, initEnd)
		if err != nil {
			return err
		}

		err = os.WriteFile(initPath, initBytes, 0644)
		if err != nil {
			return err
		}

		f, err := os.OpenFile(initPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		segStart := sidxStart + int64(sidx.sidxBoxSize) + int64(sidx.firstOffset)

		for i := 0; i < len(sidx.referencedSizes); i++ {
			sz := int64(sidx.referencedSizes[i])
			if sz <= 0 {
				break
			}
			segEnd := segStart + sz - 1
			b, err := httpRangeGet(fullFileURL, segStart, segEnd)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, bytes.NewReader(b))
			if err != nil {
				return err
			}
			segStart = segEnd + 1
		}

		if len(key) > 0 {
			cmd := exec.Command("ffmpeg", "-decryption_key", key, "-i", initPath, "-c", "copy", fmt.Sprintf("%s.mp4", id))
			err := cmd.Run()
			if err != nil {
				return err
			}
		} else {
			finalPath := fmt.Sprintf("master_%s.mp4", mediatype)
			os.Remove(finalPath)
			err = copyFile(initPath, finalPath)
			if err != nil {
				return err
			}
		}

		return nil
	}

	segmentCount := int(numberofsegments)

	fmt.Printf("Downloading init file: %s%s\n", baseurl, initmp4)

	initPath := fmt.Sprintf("./downloads/%s", initmp4)

	if _, err := os.Stat(initPath); err == nil {
		os.Remove(initPath)
	}

	err := downloadWithRetry(fmt.Sprintf("%s%s", baseurl, initmp4), initPath)
	if err != nil {
		return fmt.Errorf("error downloading init track: %v", err)
	}

	if mediaTemplate == "" {
		if len(key) > 0 {
			DecryptPlaylist(id, initmp4, key)
		} else {
			finalPath := fmt.Sprintf("master_%s.mp4", mediatype)
			if _, err := os.Stat(finalPath); err == nil {
				os.Remove(finalPath)
			}

			err = copyFile(initPath, finalPath)
			if err != nil {
				return fmt.Errorf("error creating final file: %v", err)
			}
		}
		return nil
	}

	mastertrack, err := os.OpenFile(initPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening master track: %v", err)
	}
	defer mastertrack.Close()

	var wg sync.WaitGroup
	errChan := make(chan error, segmentCount)
	files := make([]string, segmentCount)
	semaphore := make(chan struct{}, 5)

	fmt.Printf("Downloading %d segments...\n", segmentCount)

	for idx := 0; idx < segmentCount; idx++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			segNumber := startNumber + index
			segName := strings.ReplaceAll(mediaTemplate, "$RepresentationID$", adaptation)
			segName = strings.ReplaceAll(segName, "$Number$", strconv.Itoa(segNumber))

			segURL := fmt.Sprintf("%s%s", baseurl, segName)
			filename := segName

			filePath := fmt.Sprintf("./downloads/%s", filename)

			if _, err := os.Stat(filePath); err == nil {
				os.Remove(filePath)
			}

			err := downloadWithRetry(segURL, filePath)
			if err != nil {
				errChan <- fmt.Errorf("error downloading segment %d: %v", segNumber, err)
				return
			}

			files[index] = filename
		}(idx)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	for _, filename := range files {
		if filename == "" {
			continue
		}

		filePath := fmt.Sprintf("./downloads/%s", filename)
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		_, err = io.Copy(mastertrack, file)
		file.Close()
		os.Remove(filePath)

		if err != nil {
			continue
		}
	}

	if len(key) > 0 {
		DecryptPlaylist(id, initmp4, key)
	} else {
		finalPath := fmt.Sprintf("master_%s.mp4", mediatype)
		if _, err := os.Stat(finalPath); err == nil {
			os.Remove(finalPath)
		}

		err = copyFile(initPath, finalPath)
		if err != nil {
			return fmt.Errorf("error creating final file: %v", err)
		}
	}

	return nil
}

func DecryptPlaylist(id string, initmp4 string, key string) {
	cmd := exec.Command("ffmpeg", "-decryption_key", key, "-i", fmt.Sprintf("./downloads/%s", initmp4), "-c", "copy", fmt.Sprintf("%s.mp4", id))

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running ffmpeg command:", err)
		return
	}
}

func Merge(videofile string, audiofile string, kid string) {
	cmd := exec.Command("ffmpeg", "-i", videofile, "-i", audiofile, "-c:v", "copy", "-c:a", "copy", fmt.Sprintf("%s_master.mp4", kid))

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running ffmpeg command:", err)
		return
	}

	err = os.Remove(videofile)
	if err != nil {
		fmt.Println("Error deleting video file:", err)
		return
	}

	err = os.Remove(audiofile)
	if err != nil {
		fmt.Println("Error deleting audio file:", err)
		return
	}
}
