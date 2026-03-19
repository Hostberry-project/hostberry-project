package i18n

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// Manager gestiona las traducciones.
type Manager struct {
	translations        map[string]map[string]interface{}
	defaultLanguage     string
	supportedLanguages  []string
	mu                  sync.RWMutex
}

var manager *Manager
var logLanguage string = "es"

// Init inicializa el gestor de idiomas con el directorio de locales.
func Init(localesPath string) error {
	manager = &Manager{
		translations:       make(map[string]map[string]interface{}),
		defaultLanguage:    "es",
		supportedLanguages: []string{"es", "en"},
	}

	paths := []string{
		localesPath,
		"locales",
		"/opt/hostberry/locales",
		filepath.Join(filepath.Dir(os.Args[0]), "locales"),
	}

	var foundPath string
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			foundPath = path
			break
		}
	}

	if foundPath == "" {
		return fmt.Errorf("directorio de locales no encontrado")
	}

	for _, lang := range manager.supportedLanguages {
		langFile := filepath.Join(foundPath, lang+".json")
		if err := manager.loadLanguage(lang, langFile); err != nil {
			fmt.Printf("⚠️  Advertencia: No se pudo cargar %s: %v\n", langFile, err)
		}
	}

	return nil
}

func (i *Manager) loadLanguage(lang, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var translations map[string]interface{}
	if err := json.Unmarshal(data, &translations); err != nil {
		return err
	}
	i.mu.Lock()
	i.translations[lang] = translations
	i.mu.Unlock()
	return nil
}

func (i *Manager) getText(key, language string, defaultValue string) string {
	if language == "" {
		language = i.defaultLanguage
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	langTranslations, ok := i.translations[language]
	if !ok {
		langTranslations = i.translations[i.defaultLanguage]
	}
	value := i.getNestedValue(langTranslations, key)
	if value != "" {
		return value
	}
	if language != i.defaultLanguage {
		defaultTranslations := i.translations[i.defaultLanguage]
		value = i.getNestedValue(defaultTranslations, key)
		if value != "" {
			return value
		}
	}
	if defaultValue != "" {
		return defaultValue
	}
	return key
}

func (i *Manager) getNestedValue(data map[string]interface{}, key string) string {
	keys := strings.Split(key, ".")
	current := data
	for idx, k := range keys {
		if val, ok := current[k]; ok {
			if idx == len(keys)-1 {
				if str, ok := val.(string); ok {
					return str
				}
				return ""
			}
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return ""
			}
		} else {
			return ""
		}
	}
	return ""
}

func (i *Manager) getTranslations(language string) map[string]interface{} {
	if language == "" {
		language = i.defaultLanguage
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	translations, ok := i.translations[language]
	if !ok {
		translations = i.translations[i.defaultLanguage]
	}
	return translations
}

func isLanguageSupported(lang string) bool {
	if manager == nil {
		return lang == "es" || lang == "en"
	}
	for _, supported := range manager.supportedLanguages {
		if lang == supported {
			return true
		}
	}
	return false
}

// Ready indica si el gestor de idiomas está inicializado (para health checks).
func Ready() bool {
	return manager != nil
}

// GetCurrentLanguage devuelve el idioma actual según cookie, query o Accept-Language.
func GetCurrentLanguage(c *fiber.Ctx) string {
	if manager == nil {
		return "es"
	}
	if lang := c.Query("lang"); lang != "" {
		if isLanguageSupported(lang) {
			c.Cookie(&fiber.Cookie{
				Name: "lang", Value: lang, Path: "/",
				MaxAge: 365 * 24 * 60 * 60, HTTPOnly: false, SameSite: "Lax",
			})
			return lang
		}
	}
	if lang := c.Cookies("lang"); lang != "" {
		if isLanguageSupported(lang) {
			return lang
		}
	}
	acceptLang := c.Get("Accept-Language", "")
	if acceptLang != "" {
		langs := strings.Split(acceptLang, ",")
		if len(langs) > 0 {
			lang := strings.TrimSpace(strings.Split(langs[0], ";")[0])
			if len(lang) >= 2 {
				lang = lang[:2]
				if isLanguageSupported(lang) {
					return lang
				}
			}
		}
	}
	return manager.defaultLanguage
}

// T devuelve el texto traducido para la clave y el contexto.
func T(c *fiber.Ctx, key string, defaultValue string) string {
	if manager == nil {
		if defaultValue != "" {
			return defaultValue
		}
		return key
	}
	language := GetCurrentLanguage(c)
	return manager.getText(key, language, defaultValue)
}

func getSection(translations map[string]interface{}, section string) map[string]interface{} {
	if val, ok := translations[section]; ok {
		if sectionMap, ok := val.(map[string]interface{}); ok {
			return sectionMap
		}
	}
	return make(map[string]interface{})
}

// TemplateFuncs devuelve las funciones y datos para las plantillas.
func TemplateFuncs(c *fiber.Ctx) fiber.Map {
	language := GetCurrentLanguage(c)
	if manager == nil {
		return fiber.Map{
			"t": func(key string, defaultValue ...string) string {
				if len(defaultValue) > 0 {
					return defaultValue[0]
				}
				return key
			},
			"language": language, "translations": make(map[string]interface{}),
			"common": make(map[string]interface{}), "navigation": make(map[string]interface{}),
			"dashboard": make(map[string]interface{}), "auth": make(map[string]interface{}),
			"system": make(map[string]interface{}), "network": make(map[string]interface{}),
			"wifi": make(map[string]interface{}), "vpn": make(map[string]interface{}),
			"wireguard": make(map[string]interface{}), "adblock": make(map[string]interface{}),
			"settings": make(map[string]interface{}), "errors": make(map[string]interface{}),
		}
	}
	translations := manager.getTranslations(language)
	return fiber.Map{
		"t": func(key string, defaultValue ...string) string {
			def := ""
			if len(defaultValue) > 0 {
				def = defaultValue[0]
			}
			return manager.getText(key, language, def)
		},
		"language": language, "translations": translations,
		"common": getSection(translations, "common"), "navigation": getSection(translations, "navigation"),
		"dashboard": getSection(translations, "dashboard"), "auth": getSection(translations, "auth"),
		"system": getSection(translations, "system"), "network": getSection(translations, "network"),
		"wifi": getSection(translations, "wifi"), "vpn": getSection(translations, "vpn"),
		"wireguard": getSection(translations, "wireguard"), "adblock": getSection(translations, "adblock"),
		"settings": getSection(translations, "settings"), "errors": getSection(translations, "errors"),
	}
}

// SetLogLanguage establece el idioma para los logs del sistema.
func SetLogLanguage(lang string) {
	if isLanguageSupported(lang) {
		logLanguage = lang
	}
}

// GetLogLanguage devuelve el idioma actual para logs.
func GetLogLanguage() string {
	return logLanguage
}

// LogT registra un mensaje traducido.
func LogT(key string, args ...interface{}) {
	if manager == nil {
		if len(args) > 0 {
			log.Print(append([]interface{}{key}, args...)...)
			return
		}
		log.Print(key)
		return
	}
	translated := manager.getText(key, logLanguage, key)
	if len(args) > 0 {
		log.Print(fmt.Sprintf(translated, args...))
		return
	}
	log.Print(translated)
}

// LogTf registra un mensaje traducido con formato.
func LogTf(key string, args ...interface{}) {
	if manager == nil {
		if len(args) > 0 {
			log.Print(append([]interface{}{key}, args...)...)
			return
		}
		log.Print(key)
		return
	}
	translated := manager.getText(key, logLanguage, key)
	if len(args) > 0 {
		log.Print(fmt.Sprintf(translated, args...))
		return
	}
	log.Print(translated)
}

// LogTln registra un mensaje traducido con nueva línea.
func LogTln(key string, args ...interface{}) {
	if manager == nil {
		if len(args) > 0 {
			log.Println(append([]interface{}{key}, args...)...)
			return
		}
		log.Println(key)
		return
	}
	translated := manager.getText(key, logLanguage, key)
	if len(args) > 0 {
		log.Println(fmt.Sprintf(translated, args...))
		return
	}
	log.Println(translated)
}

// LogTfatal registra un mensaje fatal traducido.
func LogTfatal(key string, args ...interface{}) {
	if manager == nil {
		if len(args) > 0 {
			log.Fatal(append([]interface{}{key}, args...)...)
			return
		}
		log.Fatal(key)
		return
	}
	translated := manager.getText(key, logLanguage, key)
	if len(args) > 0 {
		log.Fatal(fmt.Sprintf(translated, args...))
		return
	}
	log.Fatal(translated)
}

// LanguageMiddleware es el middleware que establece idioma e i18n en Locals.
func LanguageMiddleware(c *fiber.Ctx) error {
	language := GetCurrentLanguage(c)
	c.Locals("language", language)
	c.Locals("i18n", TemplateFuncs(c))
	return c.Next()
}
