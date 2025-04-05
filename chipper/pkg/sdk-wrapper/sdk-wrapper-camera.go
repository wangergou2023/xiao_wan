package sdk_wrapper

import (
	"bytes"
	"image"
	"image/jpeg"
	"math/rand"
	"os"

	"github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"
)

var camStreamEnable bool = false
var camStreamClient vectorpb.ExternalInterface_CameraFeedClient

func EnableCameraStream() {
	camStreamClient, _ = Robot.Conn.CameraFeed(ctx, &vectorpb.CameraFeedRequest{})
	camStreamEnable = true
	defer camStreamClient.CloseSend()
}

func DisableCameraStream() {
	camStreamEnable = false
	camStreamClient.CloseSend()
}

func ProcessCameraStream() image.Image {
	i := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 640, Y: 360},
	})
	if camStreamEnable {
		response, _ := camStreamClient.Recv()
		imageBytes := response.GetData()
		img, _, _ := image.Decode(bytes.NewReader(imageBytes))
		return img
	} else {
		for j := range i.Pix {
			i.Pix[j] = uint8(rand.Uint32())
		}
	}

	return i.SubImage(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 640, Y: 360},
	})
}

// Enables camera, saves current image on a file in jpg format, disables camera
func GetCameraPicture() image.Image {
	if !camStreamEnable {
		EnableCameraStream()
	}
	var img = ProcessCameraStream()
	DisableCameraStream()
	return img
}

// Enables camera, saves current image on a file in jpg format, disables camera
func SaveCameraPicture(fileName string) error {
	var img = GetCameraPicture()
	f, err := os.Create(fileName)
	if err == nil {
		var opt jpeg.Options
		opt.Quality = 100
		err = jpeg.Encode(f, img, &opt)
	}
	return err
}

// Saves current image at high resolution (1280 x 720) on a file in jpg format
// This doesn't seem to work on my production Vector on Wirepod. Probably vector-cloud on the robot needs to be
// updated. But since it's a production robot I'm stuck to 1.8...
// Anyways the image is saved at the regular 360p size.

func SaveHiResCameraPicture(fileName string) error {
	img, err := GetStaticCameraPicture(true)
	if err == nil {
		f, err := os.Create(fileName)
		if err == nil {
			var opt jpeg.Options
			opt.Quality = 100
			err = jpeg.Encode(f, img, &opt)
			if err == nil {
				println("Image saved as " + fileName)
			}
		}
	}
	return err
}

// Gets a single image from camera
func GetStaticCameraPicture(hiRes bool) (image.Image, error) {
	i, err := Robot.Conn.CaptureSingleImage(
		ctx,
		&vectorpb.CaptureSingleImageRequest{
			EnableHighResolution: hiRes,
		},
	)
	if err == nil {
		var imageData = i.GetData()
		img, _, err := image.Decode(bytes.NewReader(imageData))
		return img, err
	}
	return nil, err
}
