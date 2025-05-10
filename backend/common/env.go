package common

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func GetString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func GetNumber(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	res, err := strconv.Atoi(value)
	if err != nil {
		fmt.Printf("Failed to convert env variable to int.")
		return fallback
	}
	return res
}

func TrimSuffix(s, suffix string) string {
	return strings.TrimSuffix(s, suffix)
}

func GetBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	res, err := strconv.ParseBool(value)
	if err !=nil {
		return fallback
	}
	return res
}
