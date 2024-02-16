package utils

import (
	"encoding/json"
	"log"
	"multi-player-game/consts"
)

func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func DebugPrintln(a ...any) {
	if consts.Debug {
		log.Println(a)
	}
}

func DebugPrintf(format string, a ...any) {
	if consts.Debug {
		log.Printf(format, a)
	}
}
