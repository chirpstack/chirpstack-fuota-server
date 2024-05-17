package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func GetDevicesFromC2RESTByModelId(modelId string) string {

	C2Config = OpenC2ConfigJson()

	apiURL := C2Config.C2ServerREST
	username := C2Config.Username
	password := C2Config.Password
	postData := "{\"modelId\":" + modelId + "}"

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
		fmt.Println("Error: Model not found")
	}
	//returning the devices as json string
	return string(result)
}

func GetDeviceEUIsByModelId(modelId string) []string {

	//fetch all the devices from c2 REST as json string
	jsonData := GetDevicesFromC2RESTByModelId(modelId)

	var data map[string]interface{}
	errr := json.Unmarshal([]byte(jsonData), &data)
	if errr != nil {
		fmt.Println("Error:", errr)
		return []string{}
	}

	// Access the "Device" array
	devices, ok := data["Device"].([]interface{})
	if !ok {
		fmt.Println("Error: Credentials is invalid | Device array not found in JSON")
		return []string{}
	}

	var deviceEUIs []string

	// Iterate over devices
	for _, device := range devices {

		deviceMap, ok := device.(map[string]interface{})
		if !ok {
			fmt.Println("Error: Invalid device format")
			continue
		}

		// get the device EUI
		deviceEui, _ := deviceMap["deviceCode"].(string)

		deviceEUIs = append(deviceEUIs, deviceEui)
		fmt.Println(deviceEui)
	}

	return deviceEUIs
}
