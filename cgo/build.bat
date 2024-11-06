
go build -buildmode=c-shared -ldflags="-s -w" -o dll\mqttClient.dll mqttClient.go