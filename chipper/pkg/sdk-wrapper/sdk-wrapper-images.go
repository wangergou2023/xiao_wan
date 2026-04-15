package sdk_wrapper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"time"

	"github.com/fogleman/gg"
	"github.com/nfnt/resize"
	"github.com/kercre123/wire-pod/chipper/pkg/vectorpb"
)

const IMAGE_TRANSITION_NONE = 0
const IMAGE_TRANSITION_FADE_IN = 1
const IMAGE_TRANSITION_SLIDE_LEFT = 2
const IMAGE_TRANSITION_SLIDE_RIGHT = 3
const IMAGE_TRANSITION_SLIDE_UP = 4
const IMAGE_TRANSITION_SLIDE_DOWN = 5
const IMAGE_TRANSITION_FADE_OUT = 6

const ANIMATED_GIF_SPEED_FASTEST = 0
const ANIMATED_GIF_SPEED_FAST = 0.5
const ANIMATED_GIF_SPEED_NORMAL = 1.0
const ANIMATED_GIF_SPEED_SLOW = 2.0
const ANIMATED_GIF_SPEED_SLOWER = 3.0

const VECTOR_SCREEN_WIDTH = 184
const VECTOR_SCREEN_HEIGHT = 96

var backgroundColor color.RGBA = color.RGBA{0, 0, 0, 0}
var useVectorEyeColor = false

func UseVectorEyeColorInImages(enabled bool) {
	useVectorEyeColor = enabled
}

func SetBackgroundColor(col color.RGBA) {
	backgroundColor = col
}

func TextOnImg(text string, size float64, isBold bool, color color.RGBA) []byte {
	bgImage := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: VECTOR_SCREEN_WIDTH, Y: VECTOR_SCREEN_HEIGHT},
	})
	imgWidth := bgImage.Bounds().Dx()
	imgHeight := bgImage.Bounds().Dy()
	dc := gg.NewContext(imgWidth, imgHeight)
	dc.DrawImage(bgImage, 0, 0)

	var fontName = "DroidSans"
	if isBold {
		fontName = fontName + "-Bold"
	}

	if err := dc.LoadFontFace(GetDataPath("fonts/"+fontName+".ttf"), size); err != nil {
		fmt.Println(err)
		return nil
	}

	x := float64(imgWidth / 2)
	y := float64((imgHeight / 2))
	maxWidth := float64(imgWidth) - 35.0
	if useVectorEyeColor {
		dc.SetColor(GetEyeColor())
	} else {
		dc.SetColor(color)
	}
	dc.DrawStringWrapped(text, x, y, 0.5, 0.5, maxWidth, 1.5, gg.AlignCenter)
	buf := new(bytes.Buffer)
	bitmap := ConvertPixelsToRawBitmap(dc.Image(), 100)
	for _, ui := range bitmap {
		binary.Write(buf, binary.LittleEndian, ui)
	}
	os.WriteFile("/tmp/test.raw", buf.Bytes(), 0644)
	return buf.Bytes()
}

func DataOnImg(fileName string) []byte {
	inFile, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer inFile.Close()

	src, _, err := image.Decode(inFile)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return imageOnImg(src)
}

func DataOnImgWithTransition(fileName string, transition int, pct int) []byte {
	inFile, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer inFile.Close()

	src, _, err := image.Decode(inFile)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	bgImage := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: VECTOR_SCREEN_WIDTH, Y: VECTOR_SCREEN_HEIGHT},
	})
	imgWidth := bgImage.Bounds().Dx()
	imgHeight := bgImage.Bounds().Dy()
	dc := gg.NewContext(imgWidth, imgHeight)
	dc.SetColor(backgroundColor)
	dc.Fill()
	dc.DrawImage(bgImage, 0, 0)

	dst := resize.Thumbnail(uint(imgWidth), uint(imgHeight), src, resize.Bilinear)
	x := (imgWidth - dst.Bounds().Dx()) / 2
	y := (imgHeight - dst.Bounds().Dy()) / 2
	opacity := 100
	if transition == IMAGE_TRANSITION_SLIDE_RIGHT {
		xStart := -1 * dst.Bounds().Dx()
		xEnd := (imgWidth - dst.Bounds().Dx()) / 2
		x = xStart + (xEnd-xStart)*pct/100
		//println(x)
	} else if transition == IMAGE_TRANSITION_SLIDE_LEFT {
		xStart := imgWidth + dst.Bounds().Dx()
		xEnd := (imgWidth - dst.Bounds().Dx()) / 2
		x = xStart - (xStart-xEnd)*pct/100
		//println(x)
	} else if transition == IMAGE_TRANSITION_SLIDE_DOWN {
		yStart := -1 * dst.Bounds().Dy()
		yEnd := (imgHeight - dst.Bounds().Dy()) / 2
		y = yStart + (yEnd-yStart)*pct/100
		//println(y)
	} else if transition == IMAGE_TRANSITION_SLIDE_UP {
		yStart := imgHeight + dst.Bounds().Dy()
		yEnd := (imgHeight - dst.Bounds().Dy()) / 2
		y = yStart - (yStart-yEnd)*pct/100
		//println(y)
	} else if transition == IMAGE_TRANSITION_FADE_IN {
		opacity = pct
	} else if transition == IMAGE_TRANSITION_FADE_OUT {
		opacity = 100 - pct
	}

	dc.DrawImage(dst, x, y)

	buf := new(bytes.Buffer)
	bitmap := ConvertPixelsToRawBitmap(dc.Image(), opacity)
	for _, ui := range bitmap {
		binary.Write(buf, binary.LittleEndian, ui)
	}
	os.WriteFile("/tmp/test.raw", buf.Bytes(), 0644)
	return buf.Bytes()
}

func WriteText(text string, size float64, isBold bool, duration int, blocking bool) {
	faceBytes := TextOnImg(text, size, isBold, color.RGBA{0, 255, 0, 255}) // Green
	displayFaceImage(faceBytes, duration, blocking)
}

func WriteColoredText(text string, size float64, isBold bool, color color.RGBA, duration int, blocking bool) {
	faceBytes := TextOnImg(text, size, isBold, color)
	displayFaceImage(faceBytes, duration, blocking)
}

func DisplayImage(imageFile string, duration int, blocking bool) {
	faceBytes := DataOnImg(imageFile)
	displayFaceImage(faceBytes, duration, blocking)
}

func DisplayImageWithTransition(imageFile string, duration int, transition int, numSteps int) error {
	if numSteps == 0 || transition == IMAGE_TRANSITION_NONE {
		faceBytes := DataOnImg(imageFile)
		displayFaceImage(faceBytes, duration, true)
	} else {
		if numSteps*100 > duration {
			return fmt.Errorf("Duration too short")
		}
		if transition == IMAGE_TRANSITION_FADE_OUT {
			tmpFaceBytes := DataOnImg(imageFile)
			displayFaceImage(tmpFaceBytes, duration-numSteps*33, true)
		}
		for i := 0; i <= numSteps; i++ {
			pctProgress := i * 100 / numSteps
			tmpFaceBytes := DataOnImgWithTransition(imageFile, transition, pctProgress)
			displayFaceImage(tmpFaceBytes, 33, true)
		}
		if transition != IMAGE_TRANSITION_FADE_OUT {
			tmpFaceBytes := DataOnImg(imageFile)
			displayFaceImage(tmpFaceBytes, duration-numSteps*33, true)
		}
	}
	return nil
}

func DisplayAnimatedGif(imageFile string, speed float64, loops int, repaintBackgroundAtEveryFrame bool) error {
	file, err := os.Open(imageFile)
	if err != nil {
		return fmt.Errorf("Unable to open file: %v", err)
	}
	defer file.Close()

	imageGIF, err := gif.DecodeAll(file)
	if err != nil {
		return fmt.Errorf("Unable to decode GIF: %v", err)
	}

	bgImage := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: VECTOR_SCREEN_WIDTH, Y: VECTOR_SCREEN_HEIGHT},
	})
	imgWidth := bgImage.Bounds().Dx()
	imgHeight := bgImage.Bounds().Dy()
	dc := gg.NewContext(imgWidth, imgHeight)
	dc.SetColor(backgroundColor)
	dc.Fill()
	dc.DrawImage(bgImage, 0, 0)

	for l := 0; l < loops; l++ {
		for i, img := range imageGIF.Image {
			if repaintBackgroundAtEveryFrame {
				dc.DrawImage(bgImage, 0, 0)
			}
			dst := resize.Thumbnail(uint(imgWidth), uint(imgHeight), img, resize.Bilinear)
			dc.DrawImage(dst, (imgWidth-dst.Bounds().Dx())/2, (imgHeight-dst.Bounds().Dy())/2)

			buf := new(bytes.Buffer)
			bitmap := ConvertPixelsToRawBitmap(dc.Image(), 100)
			for _, ui := range bitmap {
				binary.Write(buf, binary.LittleEndian, ui)
			}

			if speed > 0 {
				displayFaceImage(buf.Bytes(), imageGIF.Delay[i]*int(speed*10), true)
			} else {
				displayFaceImage(buf.Bytes(), 0, false)
			}
			/*
				println(fmt.Sprintf("Frame %d, size: %dx%d (resized to %dx%d) duration = %d", i,
					img.Bounds().Dx(), img.Bounds().Dy(),
					dst.Bounds().Dx(), dst.Bounds().Dy(),
					imageGIF.Delay[i]))*/
		}
	}
	return nil
}

func ConvertPixesTo16BitRGB(r uint32, g uint32, b uint32, a uint32, opacityPercentage uint16) uint16 {
	R, G, B := uint16(r/257), uint16(g/8193), uint16(b/257)

	R = R * opacityPercentage / 100
	G = G * opacityPercentage / 100
	B = B * opacityPercentage / 100

	//The format appears to be: 000bbbbbrrrrrggg

	var Br uint16 = (uint16(B & 0xF8)) << 5 // 5 bits for blue  [8..12]
	var Rr uint16 = (uint16(R & 0xF8))      // 5 bits for red   [3..7]
	var Gr uint16 = (uint16(G))             // 3 bits for green [0..2]

	out := uint16(Br | Rr | Gr)
	//println(fmt.Sprintf("%d,%d,%d -> R: %016b G: %016b B: %016b = %016b", R, G, B, Rr, Gr, Br, out))
	return out
}

func ConvertPixelsToRawBitmap(image image.Image, opacityPercentage int) []uint16 {
	imgHeight, imgWidth := image.Bounds().Max.Y, image.Bounds().Max.X
	bitmap := make([]uint16, imgWidth*imgHeight)

	for y := 0; y < imgHeight; y++ {
		for x := 0; x < imgWidth; x++ {
			/*
				// TEST CODE
				r := 0
				g := 65535 / imgWidth * (x + 1)
				b := 0
				bitmap[(y)*imgWidth+(x)] = ConvertPixesTo16BitRGB(uint32(r), uint32(g), uint32(b), 0)
			*/
			r, g, b, a := image.At(x, y).RGBA()
			if useVectorEyeColor {
				vectorEyes := GetEyeColor()
				vR := uint32(vectorEyes.R) * 255
				vG := uint32(vectorEyes.G) * 255
				vB := uint32(vectorEyes.B) * 255

				r = r * vR / 0xffff
				g = g * vG / 0xffff
				b = b * vB / 0xffff
			}
			bitmap[(y)*imgWidth+(x)] = ConvertPixesTo16BitRGB(r, g, b, a, uint16(opacityPercentage))
		}
	}
	return bitmap
}

/********************************************************************************************************/
/*                                                PRIVATE FUNCTIONS                                     */
/********************************************************************************************************/

func displayFaceImage(faceBytes []byte, duration int, blocking bool) {
	_, _ = Robot.Conn.DisplayFaceImageRGB(
		ctx,
		&vectorpb.DisplayFaceImageRGBRequest{
			FaceData:         faceBytes,
			DurationMs:       uint32(duration),
			InterruptRunning: true,
		},
	)
	if blocking {
		time.Sleep(time.Duration(duration) * time.Millisecond)
	}
}

func imageOnImg(src image.Image) []byte {
	bgImage := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: VECTOR_SCREEN_WIDTH, Y: VECTOR_SCREEN_HEIGHT},
	})
	imgWidth := bgImage.Bounds().Dx()
	imgHeight := bgImage.Bounds().Dy()
	dc := gg.NewContext(imgWidth, imgHeight)
	dc.DrawImage(bgImage, 0, 0)

	dst := resize.Thumbnail(uint(imgWidth), uint(imgHeight), src, resize.Bilinear)
	dc.DrawImage(dst, (imgWidth-dst.Bounds().Dx())/2, (imgHeight-dst.Bounds().Dy())/2)

	buf := new(bytes.Buffer)
	bitmap := ConvertPixelsToRawBitmap(dc.Image(), 100)
	for _, ui := range bitmap {
		binary.Write(buf, binary.LittleEndian, ui)
	}
	return buf.Bytes()
}
