// Tests the Skyhub server response.
package main

import (
	"encoding/json"
	"errors"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Returns the dimensions of the image from the image name description.
func getImageSizeFromName(name string) (ImageSize, error) {
	leftIndex := strings.LastIndex(name, "_")
	midIndex := strings.LastIndex(name, "x")
	rightIndex := strings.LastIndex(name, ".")
	
	if leftIndex == -1 || midIndex == -1 || rightIndex == -1 {
		return ImageSize{}, errors.New("Can't read size from image name.")
	}
	
	// Converts strings to uint (64 bits)
	width, err := strconv.ParseUint(name[leftIndex+1:midIndex], 10, 32)
	if err != nil {
		return ImageSize{}, err
	}
	height, err := strconv.ParseUint(name[midIndex+1:rightIndex], 10, 32)
	if err != nil {
		return ImageSize{}, err
	}
	
	return ImageSize{uint(width), uint(height)}, nil
}

// Downloads the Json response from the server.
// Downloads all the images from the Json response URLs, and checks their image dimensions
// against their described image name dimensions.
func TestServerJsonOutputImages(t *testing.T) {
	resp, err := http.Get("http://" + ServerAddr + "/skyhub")
	if err != nil {
		t.Fatalf("Can't get response from server %v/skyhub, %v!", ServerAddr, err);
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Can't read response body: %v!", err);
	}
	
	// Decode the provided JSON.
	var response []MongoDocument
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Can't decode response: %v!", err);
	}
	
	// Expects 30 images. 10 original images resized to 3 different dimensions (Small, Medium and Large).
	if len(response) != 30 {
		t.Fatalf("Didn't return 30 images URL. Returned %v!", len(response))	
	}
	
	// Compares the dimensions of each image with the dimensions described on the image name.
 	// Fails if any mismatch occurs.
	for _, resp := range response {
		size, err := getImageSizeFromName(resp.Name)
		if err != nil {
			t.Fatalf("Can't read size from image name. %v!", err)	
		}
		
		// Downloads images from returned URL.
		imgurl := resp.Url
		respimg, err := http.Get(imgurl)
		if err != nil {
			t.Fatalf("Can't get the image %v, %v!", imgurl, err)
    		}
		defer respimg.Body.Close()
		
		// Decodes as jpeg and gets its config.
		imgconfig, err := jpeg.DecodeConfig(respimg.Body)
		if err != nil {
			t.Fatalf("Can't decode image %v as jpeg, %v!", imgurl, err)
		}

		// Compare dimensions.   
		if imgconfig.Width != int(size.width) || imgconfig.Height != int(size.height) {
			t.Fatalf("Wrong image %v dimensions! Expected %vx%v, got %vx%v, for image %v.", imgurl, size.width, size.height, 
				 									imgconfig.Width, imgconfig.Height)
		 }
	}
}
