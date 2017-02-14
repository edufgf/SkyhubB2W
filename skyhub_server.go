// Implements the Skyhub Challenge.
// Downloads all the images from the consume endpoint
// and returns a JSON object with the name and URL for all the images 
// resized to three different dimensions (Small, Medium and Large).
//
// Uses the local filesystem to store the images and uses MongoDB to retrieve the images URLs.
//
// nfnt/resize package is used to resize the images. 
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nfnt/resize"
	"gopkg.in/mgo.v2"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type ImageSize struct {
	width, height uint
}

type ImageInfo struct {
	img image.Image
	name string
}

type Url struct {
	Url string
}

// The format of the JSON input from the endpoint we consume.
type SkyhubResponse struct {
  Images []Url
}

// Document definition for the database. Image name and its URL to
// image location on the local file system.
type MongoDocument struct {
	Name, Url string
}

var (
	smallSize = ImageSize{320, 240}
	mediumSize = ImageSize{384, 288}
	largeSize = ImageSize{640, 480}
	
	// The folder we are saving the images.
	filepathPrefix = "/home/edufgf/Desktop/B2W/skyhub/"
	
	Database = "B2W"
	Collection = "skyhub"
	
	// MongoDB address.
	DatabaseAddr = "127.0.0.1:27017"
	
	// Be sure to pick a free port.
	ServerAddr = "localhost:7366"
	
	// Source of the images we consume.
	EndpointAddr = "http://54.152.221.29/images.json"
)

// The server handle function. It displays the JSON with the resized images URLs.
func Skyhub(w http.ResponseWriter, req *http.Request) {
	db, err := connectToDatabase(DatabaseAddr)
	if err != nil {
		log.Fatal(err)
	}
	
	var results []MongoDocument
	// Pick all the documents on the database.
	// We assume that we only store valid image names and their URLs on this Database/Collection.
	db.Find(nil).All(&results)
	
	// Pretty print for JSON.
	b, err := json.MarshalIndent(results, "", "\t")
	if err != nil {
		log.Fatal(err)	
	}
	io.WriteString(w, string(b))
}

func consumeEndpoint(endpoint string) ([]Url, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Decode the provided JSON.
	var skyresp SkyhubResponse
	err = json.Unmarshal(body, &skyresp)
	if err != nil {
		return nil, err
	}
	// Return all the URLs from the JSON object.
	return skyresp.Images, nil
}

// Connects to the Database on the given URL and starts a session to the
// Database/Collection provided as global variables.
func connectToDatabase(databaseUrl string) (*mgo.Collection, error) {
	session, err := mgo.Dial(databaseUrl)
	if err != nil {
		return nil, err
	}
	c := session.DB(Database).C(Collection)
	return c, nil
}

// Returns the string after the last "/" and before the last ".".
// E.g.: http://54.152.221.29/images/b737_3.jpg
// Returns "b737_3".
func nameFromUrl(imgurl string) (error, string) {
	leftIndex := strings.LastIndex(imgurl, "/")+1
	rightIndex := strings.LastIndex(imgurl, ".")
	if leftIndex == -1 || rightIndex == -1 {
		return errors.New("Can't parse name from URL."), ""
	}
	return nil, imgurl[leftIndex:rightIndex]
}

// Make GET requests for the images URLs, decode then to jpeg format and return
// an array of ImageInfo, which gives the image (binary) and the 
// image file name (retrieved from the URL).
func getJpegImgs(imagesUrl []Url) ([]ImageInfo, error) {
	images := make([]ImageInfo, len(imagesUrl))
	for i, img := range imagesUrl {
		imgurl := img.Url
		resp, err := http.Get(imgurl)
		if err != nil {
    	return nil, err
    }
		defer resp.Body.Close()
		
		img, err := jpeg.Decode(resp.Body)
    if err != nil {
    	return nil, err
    }
    images[i].img = img
    err, images[i].name = nameFromUrl(imgurl)
    if err != nil {
			return nil, err	
		}
  }
  return images, nil
}

// Resizes the image 'img' to the dimensions provided by 'newsize'.
// Uses the Bilinear method for the resizing operation.
func resizeImg(img image.Image, newsize ImageSize) image.Image {
	return resize.Resize(newsize.width, newsize.height, img, resize.Bilinear)
}

// Saves the jpeg image to a file on the file system.
func saveImgToFile(img image.Image, filepath string) error {
	file, err := os.Create(filepath)
  if err != nil {
  	return err
  }
  return jpeg.Encode(file, img, nil)
}

// Resizes the given image to the given dimensions.
// Saves it to a local file and insert into the database the URL to access this image.
func resizeAndStoreToDB (imgInfo ImageInfo, size ImageSize, db *mgo.Collection) error {
  img := imgInfo.img
	img = resizeImg(img, size)
	// The new resized image name will contain it's dimensions.
	name := imgInfo.name + "_" + strconv.Itoa(int(size.width)) + "x" + strconv.Itoa(int(size.height)) + ".jpg"
	if err := saveImgToFile(img, filepathPrefix + name); err != nil {
		return err
	}
	// Creates the document to be inserted on the database.
	// This document will be used by database queries to retrieve the URL for this image.
	doc := MongoDocument{Name: name, Url: "http://" + ServerAddr + "/skyhub/" + name}
	nameId := struct {
		Name string
	} {
		doc.Name,
	}
	// Updates the URL for this image name, or creates new entry if image name is new.
	if _, err := db.Upsert(nameId, doc); err != nil {
		return err
	}
	return nil
}

// Resizes all the images to 3 sizes (Small, Medium and Large), 
// Saves then to local files and insert into the database URLs to access these files.
func resizeAndStoreImgsToDB (images []ImageInfo, db *mgo.Collection) error {
	for _, imgInfo := range images {
		if err := resizeAndStoreToDB(imgInfo, smallSize, db); err != nil {
			return err
		}
		if err := resizeAndStoreToDB(imgInfo, mediumSize, db); err != nil {
			return err
		}
		if err := resizeAndStoreToDB(imgInfo, largeSize, db); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	fmt.Println("Downloading images URLs...")
	imagesUrl, err := consumeEndpoint(EndpointAddr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Downloaded images URLs!\n")
	
	fmt.Println("Downloading images...")
	images, err := getJpegImgs(imagesUrl)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Downloaded images!\n")
	
	fmt.Println("Connecting to the database...")
	db, err := connectToDatabase(DatabaseAddr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to the database " + Database + "/" + Collection + "!\n")
	
	fmt.Println("Resizing all the images, saving to the local filesystem and storing the URL paths in the dabatase...")
	if err := resizeAndStoreImgsToDB (images, db); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Done!\n")
	
	// Serve the files.
	http.Handle("/skyhub/", http.StripPrefix("/skyhub/", http.FileServer(http.Dir("skyhub"))))
	
	http.HandleFunc("/skyhub", Skyhub)
	fmt.Println("Listening on " + ServerAddr + "...\n")
	log.Fatal(http.ListenAndServe(ServerAddr, nil))
}

