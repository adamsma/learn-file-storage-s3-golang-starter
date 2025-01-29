package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

type ProbeData struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {

	var b bytes.Buffer

	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams", filePath,
	)
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe error: %v", err)
	}

	var videoInfo ProbeData
	err := json.Unmarshal(b.Bytes(), &videoInfo)
	if err != nil {
		return "", fmt.Errorf("could not parse ffprobe output: %v", err)
	}

	if len(videoInfo.Streams) == 0 {
		return "", errors.New("no video streams found")
	}

	numRatio := float32(videoInfo.Streams[0].Width) / float32(videoInfo.Streams[0].Height)

	switch fmt.Sprintf("%.2f", numRatio) {
	case "0.56":
		return "9:16", nil
	case "1.78":
		return "16:9", nil
	default:
		return "other", nil
	}

}
