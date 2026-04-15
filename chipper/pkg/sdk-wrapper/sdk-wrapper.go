package sdk_wrapper

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/vector"
	"github.com/kercre123/wire-pod/chipper/pkg/vectorpb"
)

type SDKConfigData struct {
	TmpPath  string
	DataPath string
	NvmPath  string
}

var Robot *vector.Vector
var bcAssumption bool = false
var ctx context.Context
var transCfg = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // ignore SSL warnings
}

var eventStream vectorpb.ExternalInterface_EventStreamClient
var SDKConfig = SDKConfigData{"/tmp/", "data", "nvm"}

func InitSDK(serial string) error {
	var err error
	InitLanguages(LANGUAGE_ENGLISH)
	Robot, err = vector.NewEP(serial)
	if err != nil {
		log.Println(err)
		return err
	}
	ctx = context.Background()
	eventStream, err = Robot.Conn.EventStream(ctx, &vectorpb.EventRequest{})
	if err != nil {
		log.Println(err)
		return err
	}
	RefreshSDKSettings()
	return nil
}

func InitSDKForWirepod(serial string) error {
	var err error
	InitLanguages(LANGUAGE_ENGLISH)
	Robot, err = vector.NewWP(serial)
	if err != nil {
		log.Println(err)
		return err
	}
	ctx = context.Background()
	eventStream, err = Robot.Conn.EventStream(ctx, &vectorpb.EventRequest{})
	if err != nil {
		log.Println(err)
		return err
	}
	tmpPath := os.Getenv("WIREPOD_EX_TMP_PATH")
	dataPath := os.Getenv("WIREPOD_EX_DATA_PATH")
	nvmPath := os.Getenv("WIREPOD_EX_NVM_PATH")

	if tmpPath == "" {
		tmpPath = SDKConfig.TmpPath
	}
	if dataPath == "" {
		dataPath = SDKConfig.DataPath
	}
	if nvmPath == "" {
		nvmPath = SDKConfig.NvmPath
	}
	SetSDKPaths(tmpPath, dataPath, nvmPath)
	println("Completing init with paths TMP:" + tmpPath + ", DATA:" + dataPath + ", NVM:" + nvmPath)

	_, err = Robot.Conn.BatteryState(ctx, &vectorpb.BatteryStateRequest{})
	if err != nil {
		println("ERROR pingin JDOCs, likely the robot is unuthenticated")
		return err
	}
	return RefreshSDKSettings()
}

func SetSDKPaths(tmpPath string, dataPath string, nvmPath string) {
	SDKConfig.TmpPath = tmpPath
	SDKConfig.DataPath = dataPath
	SDKConfig.NvmPath = nvmPath
	_ = os.MkdirAll(tmpPath, os.ModePerm)
}

func WaitForEvent() *vectorpb.Event {
	evt, err := eventStream.Recv()
	if err != nil {
		return nil
	}
	return evt.GetEvent()
}

func AssumeBehaviorControl(priority string) {
	var controlRequest *vectorpb.BehaviorControlRequest
	if priority == "high" {
		controlRequest = &vectorpb.BehaviorControlRequest{
			RequestType: &vectorpb.BehaviorControlRequest_ControlRequest{
				ControlRequest: &vectorpb.ControlRequest{
					Priority: vectorpb.ControlRequest_OVERRIDE_BEHAVIORS,
				},
			},
		}
	} else {
		controlRequest = &vectorpb.BehaviorControlRequest{
			RequestType: &vectorpb.BehaviorControlRequest_ControlRequest{
				ControlRequest: &vectorpb.ControlRequest{
					Priority: vectorpb.ControlRequest_DEFAULT,
				},
			},
		}
	}
	go func() {
		start := make(chan bool)
		stop := make(chan bool)
		bcAssumption = true
		go func() {
			// * begin - modified from official vector-go-sdk
			r, err := Robot.Conn.BehaviorControl(
				ctx,
			)
			if err != nil {
				log.Println(err)
				return
			}

			if err := r.Send(controlRequest); err != nil {
				log.Println(err)
				return
			}

			for {
				ctrlresp, err := r.Recv()
				if err != nil {
					log.Println(err)
					return
				}
				if ctrlresp.GetControlGrantedResponse() != nil {
					start <- true
					break
				}
			}

			for {
				select {
				case <-stop:
					if err := r.Send(
						&vectorpb.BehaviorControlRequest{
							RequestType: &vectorpb.BehaviorControlRequest_ControlRelease{
								ControlRelease: &vectorpb.ControlRelease{},
							},
						},
					); err != nil {
						log.Println(err)
						return
					}
					return
				default:
					continue
				}
			}
			// * end - modified from official vector-go-sdk
		}()
		for {
			select {
			case <-start:
				for {
					if bcAssumption {
						time.Sleep(time.Millisecond * 500)
					} else {
						break
					}
				}
				stop <- true
				return
			}
		}
	}()
}

func ReleaseBehaviorControl() {
	bcAssumption = false
}

func GetRobotSerial() string {
	return Robot.Cfg.SerialNo
}

func GetTempPath() string {
	return SDKConfig.TmpPath
}

func GetTemporaryFilename(tag string, extension string, fullpath bool) string {
	tmpPath := SDKConfig.TmpPath
	tmpFile := GetRobotSerial() + "_" + tag + fmt.Sprintf("_%d", time.Now().Unix()) + "." + extension
	if fullpath {
		tmpFile = path.Join(tmpPath, tmpFile)
	}
	return tmpFile
}

func GetMyStoragePath(filename string) string {
	nvmPath := SDKConfig.NvmPath
	nvmPath = filepath.Join(nvmPath, GetRobotSerial())
	_ = os.MkdirAll(nvmPath, os.ModePerm)
	nvmPath = filepath.Join(nvmPath, filename)
	return nvmPath
}

func GetDataPath(filename string) string {
	dataPath := SDKConfig.DataPath
	var chunks []string = strings.Split(filename, "/")
	for _, chunk := range chunks {
		dataPath = filepath.Join(dataPath, chunk)
	}
	return dataPath
}

func DownloadFile(filepath string, url string) (error, string) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err, ""
	}
	defer resp.Body.Close()

	contentDisposition := resp.Header.Get("Content-Disposition")
	println(contentDisposition)
	line := strings.Split(contentDisposition, "=\"")
	fNameFromWeb := line[1]

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err, fNameFromWeb
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err, fNameFromWeb
}

/**********************************************************************************************************************/
/*                                              PRIVATE FUNCTIONS                                                     */
/**********************************************************************************************************************/

const shellToUse = "bash"

func shellout(command string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(shellToUse, "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
