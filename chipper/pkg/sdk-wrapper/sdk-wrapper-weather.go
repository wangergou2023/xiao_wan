package sdk_wrapper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/fogleman/gg"
	"image"
	"log"
	"net/http"
	"os"
)

const WEATHER_UNIT_CELSIUS = "c"
const WEATHER_UNIT_FARANHEIT = "f"

func DisplayTemperature(temp int, unit string, delay int, blocking bool) error {
	digit01 := int(temp / 10)
	digit02 := int(temp % 10)
	digit01File, err1 := os.Open(GetDataPath("images/weather/weather_temp_") + fmt.Sprintf("%d", digit01) + ".png")
	digit02File, err2 := os.Open(GetDataPath("images/weather/weather_temp_") + fmt.Sprintf("%d", digit02) + ".png")
	unitFile, err3 := os.Open(GetDataPath("images/weather/weather_celsius_indicator.png"))
	if unit == "f" {
		unitFile, err3 = os.Open(GetDataPath("images/weather/weather_fahrenheit_indicator.png"))
	}
	signFile, err4 := os.Open(GetDataPath("images/weather/weather_negative_indicator.png"))

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return fmt.Errorf("Unable to open weather image files")
	}

	defer digit01File.Close()
	defer digit02File.Close()
	defer unitFile.Close()
	defer signFile.Close()

	var digit01Img, digit02Img, unitImg, signImg image.Image

	digit01Img, _, err1 = image.Decode(digit01File)
	digit02Img, _, err2 = image.Decode(digit02File)
	unitImg, _, err1 = image.Decode(unitFile)
	signImg, _, err1 = image.Decode(signFile)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return fmt.Errorf("Unable to decode weather image files")
	}

	w0 := signImg.Bounds().Dx()
	w1 := digit01Img.Bounds().Dx()
	h1 := digit01Img.Bounds().Dy()
	w2 := digit02Img.Bounds().Dx()
	w3 := unitImg.Bounds().Dx()

	if temp >= 0 {
		w0 = 0
	}
	if temp < 10 && temp > -10 {
		w1 = 0
	}

	var x = int((VECTOR_SCREEN_WIDTH - (w0 + w1 + w2 + w3)) / 2)
	var y = int((VECTOR_SCREEN_HEIGHT - (h1)) / 2)

	println(fmt.Sprintf("%d,%d,%d,%d,%d,%d", w0, w1, w2, w3, x, y))

	bgImage := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: VECTOR_SCREEN_WIDTH, Y: VECTOR_SCREEN_HEIGHT},
	})
	dc := gg.NewContext(VECTOR_SCREEN_WIDTH, VECTOR_SCREEN_HEIGHT)
	dc.DrawImage(bgImage, 0, 0)

	if temp < 0 {
		dc.DrawImage(signImg, x, y)
	}
	x = x + w0
	if temp >= 10 || temp <= -10 {
		dc.DrawImage(digit01Img, x, y)
	}
	x = x + w1
	dc.DrawImage(digit02Img, x, y)
	x = x + w2
	dc.DrawImage(unitImg, x, y)

	buf := new(bytes.Buffer)
	bitmap := ConvertPixelsToRawBitmap(dc.Image(), 100)
	for _, ui := range bitmap {
		binary.Write(buf, binary.LittleEndian, ui)
	}

	displayFaceImage(buf.Bytes(), delay, blocking)
	return nil
}

func DisplayCondition(condition string, iconCode string, duration int, blocking bool) {
	imgUrl := "http://openweathermap.org/img/wn/" + iconCode + "@2x.png"
	image, err := loadImageFromUrl(imgUrl)
	if err == nil {
		faceBytes := imageOnImg(image)
		displayFaceImage(faceBytes, duration, blocking)
	}
}

func loadImageFromUrl(url string) (image.Image, error) {
	res, err := http.Get(url)

	if err != nil {
		log.Fatalf("http.Get -> %v", err)
		return nil, err
	}
	defer res.Body.Close()
	m, _, err2 := image.Decode(res.Body)
	return m, err2
}
