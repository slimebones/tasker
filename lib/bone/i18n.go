package bone

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// {locale: {key: value}}
var translationMap = map[string]map[string]string{}
var translationLocale string = "en"

// Register a translation from a CSV file.
// CSV file structure:
// key(string),text(string)
//
// This function can be called many times, each new call the old matching
// entries will be overwritten.
//
// Text may contain placeholders in form of `%` to accept incoming value,
// which will always be converted to string.
//
// For list of locales refer to https://docs.godotengine.org/en/4.3/tutorials/i18n/locales.html
func TrLoadCsv(path string, locale string, delimiter rune) bool {
	locale = strings.ToLower(locale)

	file, e := os.Open(path)
	if e != nil {
		return false
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = delimiter
	records, e := reader.ReadAll()
	if e != nil {
		return false
	}

	localeMap, ok := translationMap[locale]
	if !ok {
		localeMap = map[string]string{}
		translationMap[locale] = localeMap
	}

	for i, record := range records {
		if len(record) != 2 {
			return false
		}
		if i == 0 {
			continue
		}
		localeMap[strings.TrimSpace(record[0])] = strings.TrimSpace(record[1])
	}

	return true
}

func Tr(key string) string {
	t, ok := TrOrError(key)
	if !ok {
		return key
	}
	return t
}

func Tr_Code(code int) string {
	return Tr(fmt.Sprintf("CODE_%d", code))
}

func TrOrError(key string) (string, bool) {
	key = strings.ToUpper(key)
	localeMap, ok := translationMap[translationLocale]
	if !ok {
		return "", false
	}
	text, ok := localeMap[strings.ToUpper(key)]
	if !ok {
		return "", false
	}
	return text, true
}
