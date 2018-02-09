package i18n

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

// LanguageTagRegex Matches language tags like en-US, and zh-Hans-CN.
// Language tags are case-insensitive.
var LanguageTagRegex = regexp.MustCompile(`[a-zA-Z]{2,}([\-_][a-zA-Z]{2,})+`)

// Translator translates messages.
type Translator struct {
	Bundle       *Bundle
	LanguageTags []string
}

// NewTranslator returns a translator that looks up translations
// in the bundle according to the order of language tags found in preferences.
//
// It can parse languages from Accept-Language headers (RFC 2616),
// but it assumes weights are monotonically decreasing.
func NewTranslator(bundle *Bundle, prefs string) *Translator {
	translator := &Translator{
		Bundle:       bundle,
		LanguageTags: []string{},
	}

	langTags := LanguageTagRegex.FindAllString(prefs, -1)
	var tags []string
	for _, langTag := range langTags {
		tags = append(tags, expandTag(langTag)...)
	}
	translator.LanguageTags = dedupe(tags)
	return translator
}

func expandTag(tag string) []string {
	tag = strings.TrimSpace(tag)
	tag = strings.ToLower(tag)
	tags := []string{tag}
	for i := len(tag) - 1; i > 0; {
		r, size := utf8.DecodeLastRuneInString(tag[:i])
		i -= size
		if r == '-' || r == '_' {
			tags = append(tags, tag[:i])
		}
	}
	return tags
}

func dedupe(strs []string) []string {
	found := make(map[string]struct{}, len(strs))
	deduped := make([]string, 0, len(strs))
	for _, str := range strs {
		if _, ok := found[str]; !ok {
			found[str] = struct{}{}
			deduped = append(deduped, str)
		}
	}
	return deduped
}

// Translate iterates through language tags to find the first non-empty translation in the bundle.
// It returns the default translation if no other translation is found.
func (t *Translator) Translate(id, defaultTranslation string, args ...interface{}) string {
	if len(args) > 2 {
		panic("too many args passed to Localize")
	}
	for _, langTag := range t.LanguageTags {
		translations := t.Bundle.Translations[langTag]
		if translations == nil {
			continue
		}
		translation := translations[id]
		if translation == nil {
			continue
		}
		pluralRule := t.Bundle.PluralRules[langTag]
		if pluralRule == nil {
			continue
		}
		pluralCount, data := parseArgs(args)
		pluralForm, err := pluralRule.PluralForm(pluralCount)
		if err != nil {
			return fmt.Sprintf("[ERR][%s] %s", id, err.Error())
		}
		translated := translation.Translate(pluralForm, data)
		if translated == "" {
			continue
		}
		return translated
	}
	return defaultTranslation
}

func parseArgs(args []interface{}) (count interface{}, data interface{}) {
	if argc := len(args); argc > 0 {
		if isNumber(args[0]) {
			count = args[0]
			if argc > 1 {
				data = args[1]
			}
		} else {
			data = args[0]
		}
	}

	if count != nil {
		if data == nil {
			data = map[string]interface{}{"Count": count}
		} else {
			dataMap := toMap(data)
			dataMap["Count"] = count
			data = dataMap
		}
	} else {
		dataMap := toMap(data)
		if c, ok := dataMap["Count"]; ok {
			count = c
		}
	}
	return
}

func isNumber(n interface{}) bool {
	switch n.(type) {
	case int, int8, int16, int32, int64, string:
		return true
	}
	return false
}

func toMap(input interface{}) map[string]interface{} {
	if data, ok := input.(map[string]interface{}); ok {
		return data
	}
	v := reflect.ValueOf(input)
	switch v.Kind() {
	case reflect.Ptr:
		return toMap(v.Elem().Interface())
	case reflect.Struct:
		return structToMap(v)
	default:
		return nil
	}
}

// Converts the top level of a struct to a map[string]interface{}.
// Code inspired by github.com/fatih/structs.
func structToMap(v reflect.Value) map[string]interface{} {
	out := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			// skip unexported field
			continue
		}
		out[field.Name] = v.FieldByName(field.Name).Interface()
	}
	return out
}
