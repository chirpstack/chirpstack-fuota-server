package main

import (
	"github.com/chirpstack/chirpstack-fuota-server/v4/cmd/chirpstack-fuota-server/cmd"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
	//"github.com/chirpstack/chirpstack-fuota-server/v4/cmd/chirpstack-fuota-server/cmd"
)

// grpcLogger implements a wrapper around the logrus Logger to make it
// compatible with the grpc LoggerV2. It seems that V is not (always)
// called, therefore the Info* methods are overridden as we want to
// log these as debug info.
type grpcLogger struct {
	*log.Logger
}

func (gl *grpcLogger) V(l int) bool {
	level, ok := map[log.Level]int{
		log.DebugLevel: 0,
		log.InfoLevel:  1,
		log.WarnLevel:  2,
		log.ErrorLevel: 3,
		log.FatalLevel: 4,
	}[log.GetLevel()]
	if !ok {
		return false
	}

	return l >= level
}

func (gl *grpcLogger) Info(args ...interface{}) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug(args...)
	}
}

func (gl *grpcLogger) Infoln(args ...interface{}) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug(args...)
	}
}

func (gl *grpcLogger) Infof(format string, args ...interface{}) {
	if log.GetLevel() == log.DebugLevel {
		log.Debugf(format, args...)
	}
}

func init() {
	grpclog.SetLoggerV2(&grpcLogger{log.StandardLogger()})
}

var version string // set by the compiler

func main() {
	cmd.Execute(version)
	// api.InitConnection()
	// api.SendMessage("{\"msg_type\":\"C2OP\",\"filter\":\"{\"id\":0,\"startingRow\":-1,\"maxRecordCount\":-1,\"orderByProperty\":\"reportedOn\",\"filterDeleted\":false,\"lastModified\":0,\"dateRange\":{\"id\":0,\"duration\":0,\"selection\":0,\"fromDate\":0,\"toDate\":0},\"lastSyncTimeServer\":1709616352182,\"fullImport\":false}\"}")
	// api.ReceiveMessage()
	// api.CloseConnection()

	// api.GetDeviceEUIsByModelId("1")
}
