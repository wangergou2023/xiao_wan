package vector

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digital-dream-labs/hugh/grpc/client"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	"google.golang.org/grpc"
	"gopkg.in/ini.v1"
)

// Vector is the struct containing info about Vector
type Vector struct {
	Conn vectorpb.ExternalInterfaceClient
	Cfg  options
}

func (v *Vector) GetIPAddress() string {
	targetIP := strings.Split(v.Cfg.Target, ":")[0]
	return targetIP
}

// New returns either a vector struct, or an error on failure
func New(opts ...Option) (*Vector, error) {
	cfg := options{}

	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Target == "" || cfg.Token == "" {
		return nil, fmt.Errorf("configuration options missing")
	}

	c, err := client.New(
		client.WithTarget(cfg.Target),
		client.WithInsecureSkipVerify(),
		client.WithDialopts(
			grpc.WithPerRPCCredentials(
				tokenAuth{
					token: cfg.Token,
				},
			),
		),
	)
	if err != nil {
		return nil, err
	}
	if err := c.Connect(); err != nil {
		return nil, err
	}

	r := Vector{
		Conn: vectorpb.NewExternalInterfaceClient(c.Conn()),
		Cfg:  cfg,
	}

	return &r, nil
}

type RobotSDKInfoStore struct {
	GlobalGUID string `json:"global_guid"`
	Robots     []struct {
		Esn       string `json:"esn"`
		IPAddress string `json:"ip_address"`
		GUID      string `json:"guid"`
		Activated bool   `json:"activated"`
	} `json:"robots"`
}

// NewWP returns either a vector struct for wirepod pod vector, or an error on failure
// This function assumes you are working with Wirepod, that saves in "./jdocs/botSdkInfo.json" a JSON file with the
// configuration info needed
func NewWP(serial string) (*Vector, error) {
	if serial == "" {
		log.Fatal("please use the -serial argument and set it to your robots serial number")
		return nil, fmt.Errorf("Configuration options missing")
	}

	cfg := options{}
	wirepodPath := os.Getenv("WIREPOD_HOME")
	if len(wirepodPath) == 0 {
		wirepodPath = "."
	}
	botSdkInfoFile := filepath.Join(wirepodPath, "chipper/jdocs/botSdkInfo.json")
	jsonBytes, err := os.ReadFile(botSdkInfoFile)
	if err != nil {
		log.Println("vector-go-sdk error: Error opening " + botSdkInfoFile + ", likely doesn't exist")
		return nil, err
	}
	var robotSDKInfo RobotSDKInfoStore
	json.Unmarshal(jsonBytes, &robotSDKInfo)
	matched := false
	for _, robot := range robotSDKInfo.Robots {
		if strings.TrimSpace(strings.ToLower(robot.Esn)) == strings.TrimSpace(strings.ToLower(serial)) {
			cfg.Target = robot.IPAddress + ":443"
			cfg.Token = robot.GUID
			matched = true
		}
	}
	if !matched {
		log.Println("vector-go-sdk error: serial did not match any bot in bot json")
		return nil, errors.New("vector-go-sdk error: serial did not match any bot in bot json")
	}
	c, err := client.New(
		client.WithTarget(cfg.Target),
		client.WithInsecureSkipVerify(),
	)
	if err != nil {
		return nil, err
	}
	if err := c.Connect(); err != nil {
		return nil, err
	}

	cfg.SerialNo = serial
	log.Println("Creating EP connection with robot on address " + cfg.Target + ", serialNo " + cfg.SerialNo + ", GUID " + robotSDKInfo.GlobalGUID)

	return New(
		WithTarget(cfg.Target),
		WithSerialNo(cfg.SerialNo),
		WithToken(cfg.Token),
	)
}

// NewEP returns either a vector struct for escape pod vector, or an error on failure
// This function assumes you are working with the old Python SDK, that saves in ".anki_vector" a .ini file with the
// configuration info needed
func NewEP(serial string) (*Vector, error) {
	if serial == "" {
		log.Fatal("please use the -serial argument and set it to your robots serial number")
		return nil, fmt.Errorf("Configuration options missing")
	}

	cfg := options{}

	homedir, _ := os.UserHomeDir()
	dirname := filepath.Join(homedir, ".anki_vector", "sdk_config.ini")

	if initData, _ := ini.Load(dirname); initData != nil {
		sec, _ := initData.GetSection(serial)
		sec.MapTo(&cfg)
	} else {
		return nil, fmt.Errorf("INI file missing")
	}

	cfg.SerialNo = serial
	cfg.Target = fmt.Sprintf("%s:443", cfg.Target)

	println(cfg.SerialNo)
	println(cfg.Target)
	println(cfg.Token)
	println(cfg.CertPath)
	println(cfg.RobotName)

	c, err := client.New(
		client.WithTarget(cfg.Target),
		client.WithInsecureSkipVerify(),
	)
	if err != nil {
		return nil, err
	}
	if err := c.Connect(); err != nil {
		return nil, err
	}

	return New(
		WithSerialNo(cfg.SerialNo),
		WithTarget(cfg.Target),
		WithToken(cfg.Token),
	)
}

// NewWpExternal returns either a vector struct for wirepod vector, or an error on failure
// This function assumes that you are working with a wirepod setup (i.e. your robot is activated on wirepod, and you
// have wirepod running on your local network)
// it only needs the serial number and will retrieve rest of the data needed from wirepod API
func NewWpExternal(serial string) (*Vector, error) {
	cfg := options{}

	const apiUrl = "http://escapepod.local/api-sdk/get_sdk_info"

	httpClient := http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest(http.MethodGet, apiUrl, nil)
	res, getErr := httpClient.Do(req)
	if getErr != nil {
		log.Println("Error getting data from wirepod API", getErr)
		return nil, getErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Println("unable to read data from the wirepod API")
		return nil, readErr
	}

	robotSDKInfo := RobotSDKInfoStore{}
	jsonErr := json.Unmarshal(body, &robotSDKInfo)

	if jsonErr != nil {
		log.Println("error reading JSON data")
		return nil, jsonErr
	}

	matched := false
	for _, robot := range robotSDKInfo.Robots {
		if strings.TrimSpace(strings.ToLower(robot.Esn)) == strings.TrimSpace(strings.ToLower(serial)) {
			cfg.Target = robot.IPAddress + ":443"
			cfg.Token = robot.GUID
			matched = true
			break
		}
	}
	if !matched {
		log.Println("vector-go-sdk error: serial did not match any bot in wirepod API")
		return nil, errors.New("vector-go-sdk error: serial did not match any bot in wirepod API")
	}

	c, err := client.New(
		client.WithTarget(cfg.Target),
		client.WithInsecureSkipVerify(),
	)
	if err != nil {
		return nil, err
	}
	if err := c.Connect(); err != nil {
		return nil, err
	}

	return New(
		WithSerialNo(cfg.SerialNo),
		WithTarget(cfg.Target),
		WithToken(cfg.Token),
	)
}
