package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

type C2ConfigType struct {
	C2ServerREST string `json:"c2serverREST"`
	C2ServerWS   string `json:"c2serverWS"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Frequency    int    `json:"frequency"`
}

var C2Config C2ConfigType
var conn *websocket.Conn
var err error

func InitConnection() {

	//open C2Config.json file
	C2Config = OpenC2ConfigJson()

	//C2Config.json properties can be accessed by using C2Config.<property_name>
	username := C2Config.Username
	Password := C2Config.Password

	//creating authentication string
	authString := fmt.Sprintf("%s:%s", username, Password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// websocketURL := C2Config.C2ServerWS + encodedAuth + "/true" // Device authentication
	websocketURL := C2Config.C2ServerWS //User authentication

	// Basic Authorization header
	headers := make(http.Header)
	// headers.Set("Device", "Basic "+encodedAuth) //Device authentication
	headers.Set("Authorization", "Basic "+encodedAuth) //User authentication

	// Establish WebSocket connection
	conn, _, err = websocket.DefaultDialer.Dial(websocketURL, headers)
	if err != nil {
		log.Fatal("C2 Websocket server is offline:", err)
	}

	fmt.Println("Websocket Connection Established")

}

func SendMessage(message string) {

	// prepare request message string
	// message = fmt.Sprintf(`{"msg_type": "req_bonded_devices", "device": "%s", "ls": 0}`, C2Config.Username)

	err := conn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatal("Write error:", err)
	}
	fmt.Println("Websocket Message Sent: " + message)
}

func ReceiveMessage() {

	// Handle incoming messages from the WebSocket server
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Fatal("Read error:", err)
		}
		fmt.Println("Websocket Message received: " + string(message))

		//Add condition here to stop listening for incoming messages
		break
	}
}

func CloseConnection() {
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Println("Write close error:", err)
	}

	conn.Close()
	fmt.Println("Websocket Connection Closed")
}

func OpenC2ConfigJson() C2ConfigType {
	//open C2Config.json file
	path := "C2Config.json"

	config := C2ConfigType{}

	c2Data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error opening c2.json file:", err)
	}

	err = json.Unmarshal(c2Data, &config)
	if err != nil {
		fmt.Println("Error decoding c2.json file:", err)
	}

	return config
}
