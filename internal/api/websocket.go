package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"

	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/applayer/multicastsetup"
	fuota "github.com/chirpstack/chirpstack-fuota-server/v4/api/go"
)

// type Config struct {
// 	Username     string `toml:"username"`
// 	Password     string `toml:"password"`
// 	C2ServerWS   string `toml:"c2serverWS"`
// 	C2ServerREST string `toml:"c2serverREST"`
// 	Frequency    int    `toml:"frequency"`
// }

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

type Device struct {
	DeviceEUI  string `json:"deviceEUI"`
	DeviceName string `json:"deviceName"`
	// Add other device-related fields here.
}

type ResponseData struct {
	FirmwareVersion string   `json:"firmwareVersion"`
	Devices         []Device `json:"devices"`
}

// var C2Config = OpenC2ConfigToml()
var WSConn *websocket.Conn
var GrpcConn *grpc.ClientConn
var err error

func InitGrpcConnection() {
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}

	GrpcConn, err = grpc.Dial("localhost:8070", dialOpts...)
	if err != nil {
		panic(err)
	}
	fmt.Println("")
	log.Println("Grpc Connection Established")
}

func InitWSConnection() {

	// username := C2Config.Username
	// Password := C2Config.Password

	username := getC2Username()
	password := getC2Password()

	//creating authentication string
	authString := fmt.Sprintf("%s:%s", username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	websocketURL := getC2serverUrl() + encodedAuth + "/true" // Device authentication
	// websocketURL := getC2serverUrl() //User authentication

	headers := make(http.Header)
	headers.Set("Device", "Basic "+encodedAuth) //Device authentication
	// headers.Set("Authorization", "Basic "+encodedAuth) //User authentication

	WSConn, _, err = websocket.DefaultDialer.Dial(websocketURL, headers)
	if err != nil {
		log.Fatal("C2 Websocket server is offline:", err)
	}

	fmt.Println("")
	log.Println("Websocket Connection Established")
}

func CloseConnection() {
	err = WSConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Println("Write close error:", err)
	}

	WSConn.Close()
	fmt.Println("")
	log.Println("Websocket Connection Closed")
}

func SendMessage(message string) {
	// prepare request message string
	// message = fmt.Sprintf(`{"msg_type": "req_bonded_devices", "device": "%s", "ls": 0}`, C2Config.Username)

	err := WSConn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatal("Write error:", err)
	}
	fmt.Println("")
	log.Println("Websocket Message Sent: " + message)
}

func ReceiveMessage() {
	for {
		_, message, err := WSConn.ReadMessage()
		if err != nil {
			log.Fatal("Read error:", err)
		}
		fmt.Println("\n")
		log.Println("Websocket Message received: " + string(message))
		handleMessage(string(message))
		//Add condition here to stop listening for incoming messages
		break
	}
}

func ReceiveMessageDummy() {
	for {
		// _, message, err := WSConn.ReadMessage()
		// if err != nil {
		// 	log.Fatal("Read error:", err)
		// }
		dummyResponseJson := `[{"firmwareVersion":"1.0.0","devices":[{"deviceEUI":"500a57774bed650e","deviceName":"DeviceA","location":"Building 1, Floor 2"},{"deviceEUI":"deb16f73fe59744f","deviceName":"DeviceB","location":"Building 1, Floor 3"}]},{"firmwareVersion":"1.1.0","devices":[{"deviceEUI":"bcccfa1e80d50cff","deviceName":"DeviceC","location":"Building 2, Floor 1"},{"deviceEUI":"f6efb27acc31cb64","deviceName":"DeviceD","location":"Building 2, Floor 2"}]}]`
		fmt.Println("\n")
		log.Println("Dummy Message received: \n" + dummyResponseJson)
		handleMessage(dummyResponseJson)
		//Add condition here to stop listening for incoming messages
		break
	}
}

func handleMessage(message string) {
	var firmwares []ResponseData
	if err := json.Unmarshal([]byte(message), &firmwares); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	// Process each firmware version
	for _, firmware := range firmwares {
		createDeploymentRequest(firmware)
	}
}

func createDeploymentRequest(firmware ResponseData) {
	//get applicationId,regionId, firmware payload from C2
	// var applicationId string = C2Config.ApplicationId
	var applicationId string = getApplicationId()
	var regionId int = 6136
	var payload string = ""

	go UpdateFirmware(firmware.FirmwareVersion, firmware.Devices, applicationId, regionId, payload)
}

func UpdateFirmware(firmwareVersion string, devices []Device, applicationId string, regionId int, payload string) {
	mcRootKey, err := multicastsetup.GetMcRootKeyForGenAppKey(lorawan.AES128Key{0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("FirmwareVersion: " + firmwareVersion)

	client := fuota.NewFuotaServerServiceClient(GrpcConn)

	resp, err := client.CreateDeployment(context.Background(), &fuota.CreateDeploymentRequest{
		Deployment: &fuota.Deployment{
			ApplicationId:                     applicationId,
			Devices:                           GetDeploymentDevices(mcRootKey, devices),
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

	// // log.Printf("deployment request sent: %s\n", id)

	// ticker := time.NewTicker(1 * time.Minute)
	// defer ticker.Stop()

	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		GetStatus(id)
	// 	}
	// }
}

func GetDeploymentDevices(mcRootKey lorawan.AES128Key, devices []Device) []*fuota.DeploymentDevice {

	var deploymentDevices []*fuota.DeploymentDevice
	for _, device := range devices {
		fmt.Println("	device eui: " + device.DeviceEUI)
		deploymentDevices = append(deploymentDevices, &fuota.DeploymentDevice{
			DevEui:    device.DeviceEUI,
			McRootKey: mcRootKey.String(),
		})
	}

	return deploymentDevices
}

func GetStatus(id uuid.UUID) {

	client := fuota.NewFuotaServerServiceClient(GrpcConn)

	resp, err := client.GetDeploymentStatus(context.Background(), &fuota.GetDeploymentStatusRequest{
		Id: id.String(),
	})

	if err != nil {
		panic(err)
	}

	log.Printf("deployment status: %s\n", resp.EnqueueCompletedAt)
}

// func OpenC2ConfigToml() Config {
// 	var cfg Config
// 	if _, err := toml.DecodeFile("config.toml", &cfg); err != nil {
// 		log.Println("Error reading config file:", err)
// 	}
// 	return cfg
// }

func Scheduler() {
	ticker := time.NewTicker(time.Duration(getC2Frequency()) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			CheckForFirmwareUpdate()
		}
	}
}

func CheckForFirmwareUpdate() {
	SendMessage("")
	ReceiveMessageDummy()
	// go ReceiveMessage()
}

func getApplicationId() string {

	viper.SetConfigName("c2int_runtime_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_runtime_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_runtime_config.toml file: %v", err)
		}
	}

	applicationId := viper.GetString("chirpstack.application.id")
	if applicationId == "" {
		log.Fatal("Application id not found in c2int_runtime_config.toml file")
	}

	return applicationId
}

func getC2Username() string {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	username := viper.GetString("c2App.username")
	if username == "" {
		log.Fatal("username not found in c2int_boot_config.toml file")
	}

	return username
}

func getC2Password() string {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	password := viper.GetString("c2App.password")
	if password == "" {
		log.Fatal("password not found in c2int_boot_config.toml file")
	}

	return password
}

func getC2serverUrl() string {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	url := viper.GetString("c2App.serverUrl")
	if url == "" {
		log.Fatal("serverUrl not found in c2int_boot_config.toml file")
	}

	return url
}

func getC2Frequency() int {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	frequency := viper.GetInt("c2App.frequency")
	if frequency == 0 {
		log.Fatal("frequency not found in c2int_boot_config.toml file")
	}

	return frequency
}
