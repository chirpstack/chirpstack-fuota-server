package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"

	"github.com/BurntSushi/toml"
	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/applayer/multicastsetup"
	fuota "github.com/chirpstack/chirpstack-fuota-server/v4/api/go"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/api"
)

type Config struct {
	Username     string `toml:"username"`
	Password     string `toml:"password"`
	C2ServerWS   string `toml:"c2serverWS"`
	C2ServerREST string `toml:"c2serverREST"`
	Frequency    int    `toml:"frequency"`
}

var C2Config = OpenC2ConfigToml()

var region = map[int]fuota.Region{
	6134: fuota.Region_AU915,
	6135: fuota.Region_CN779,
	6136: fuota.Region_EU868,
	6137: fuota.Region_IN865,
	6138: fuota.Region_EU433,
	6139: fuota.Region_ISM2400,
	6140: fuota.Region_KR920,
	6141: fuota.Region_AS923,
	6142: fuota.Region_US915,
}

func main() {
	Scheduler()
}

func Scheduler() {
	ticker := time.NewTicker(time.Duration(C2Config.Frequency) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			CheckForFirmwareUpdate()
		}
	}
}

func CheckForFirmwareUpdate() {

	apiURL := C2Config.C2ServerREST
	username := C2Config.Username
	password := C2Config.Password
	postData := "{}"

	//creating authentication string
	authString := fmt.Sprintf("%s:%s", username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	//post request to c2 server
	req, err := http.NewRequest("POST", apiURL, bytes.NewBufferString(postData))
	if err != nil {
		fmt.Println("Error creating request:", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error decoding response:", err)
	}

	if result == nil {
		fmt.Println("Error: service not found")
	}

	var data map[string]interface{}
	errr := json.Unmarshal([]byte(string(result)), &data)
	if errr != nil {
		fmt.Println("Error:", errr)
		return
	}
	// get the device EUI
	modelId, _ := data["modelId"].(string)
	firmwareVersion, _ := data["version"].(string)
	updateFirmware, _ := data["update"].(bool)
	applicationId, _ := data["applicationId"].(string)
	regionId, _ := data["regionId"].(int)
	payload, _ := data["payload"].(string)

	if updateFirmware == true {
		UpdateFirmware(modelId, firmwareVersion, applicationId, regionId, payload)
	}
}

func UpdateFirmware(modelId string, firmwareVersion string, applicationId string, regionId int, payload string) {
	mcRootKey, err := multicastsetup.GetMcRootKeyForGenAppKey(lorawan.AES128Key{0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		log.Fatal(err)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial("localhost:8070", dialOpts...)
	if err != nil {
		panic(err)
	}

	client := fuota.NewFuotaServerServiceClient(conn)

	resp, err := client.CreateDeployment(context.Background(), &fuota.CreateDeploymentRequest{
		Deployment: &fuota.Deployment{
			ApplicationId:                     applicationId,
			Devices:                           GetDeploymentDevices(mcRootKey, modelId),
			MulticastGroupType:                fuota.MulticastGroupType_CLASS_C,
			MulticastDr:                       5,
			MulticastFrequency:                868100000,
			MulticastGroupId:                  0,
			MulticastTimeout:                  6,
			MulticastRegion:                   region[regionId],
			UnicastTimeout:                    ptypes.DurationProto(60 * time.Second),
			UnicastAttemptCount:               1,
			FragmentationFragmentSize:         50,
			Payload:                           []byte(payload),
			FragmentationRedundancy:           1,
			FragmentationSessionIndex:         0,
			FragmentationMatrix:               0,
			FragmentationBlockAckDelay:        1,
			FragmentationDescriptor:           []byte{0, 0, 0, 0},
			RequestFragmentationSessionStatus: fuota.RequestFragmentationSessionStatus_AFTER_SESSION_TIMEOUT,
		},
	})
	if err != nil {
		panic(err)
	}

	var id uuid.UUID
	copy(id[:], resp.GetId())

	fmt.Printf("deployment created: %s\n", id)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			GetStatus(id)
		}
	}
}

func GetDeploymentDevices(mcRootKey lorawan.AES128Key, modelId string) []*fuota.DeploymentDevice {
	deviceEuis := api.GetDeviceEUIsByModelId(modelId)
	// applicationId := api.GetApplicationId()

	var deploymentDevices []*fuota.DeploymentDevice
	for _, eui := range deviceEuis {
		deploymentDevices = append(deploymentDevices, &fuota.DeploymentDevice{
			DevEui:    eui,
			McRootKey: mcRootKey.String(),
		})
	}

	return deploymentDevices
}

func GetStatus(id uuid.UUID) {
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial("localhost:8070", dialOpts...)
	if err != nil {
		panic(err)
	}
	client := fuota.NewFuotaServerServiceClient(conn)

	resp, err := client.GetDeploymentStatus(context.Background(), &fuota.GetDeploymentStatusRequest{
		Id: id.String(),
	})

	if err != nil {
		panic(err)
	}

	fmt.Printf("deployment status: %s\n", resp.EnqueueCompletedAt)
}

func OpenC2ConfigToml() Config {
	var cfg Config
	if _, err := toml.DecodeFile("config.toml", &cfg); err != nil {
		fmt.Println("Error reading config file:", err)
	}
	return cfg
}
