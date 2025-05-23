package bone

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/go-ini/ini"
)

const (
	OK = iota
	ERROR
)

type Init_Args struct {
	Project_Name string
}

var init_args Init_Args

// Inits all bone stuff. Should be called at startup in main-like function.
// Noone wants `Bone` to fail initialization, so we only have `Init` in
// the API.
//
// Define `flag` arguments *before* calling to this function, since internally
// it calls `flag.Parse()`.
func Init(project_name string) int {
	if project_name == "" {
		Log_Error("Unset project name")
		return 1
	}

	var userdirflag *string
	var cfgpath string
	// We don't support args passing in testing mode, so we just initialize
	// empty strings. Vardir will be targeted to default location and mode will
	// be retrieved from `var/cfg`.
	if Testing() {
		tmp := os.TempDir()
		// Testing mode has special directory to not interfere with standard
		// var files.
		userdirflag = Atop(path.Join(tmp, project_name, "testing"))
		// Each test temporary testing directory is re-created.
		Mkdir(*userdirflag)
		e := os.RemoveAll(*userdirflag)
		if e != nil {
			Log_Error("In bone, failed to clear tmp directory")
			return 1
		}
		// Though if for standard mode we use cfg located at var dir, for
		// testing mode our vardir becomes temporary, so we grab our static
		// configuration from the current working directory, which should be
		// the repository root, since it's the only right place from where unit
		// testing should be started.
		cfgpath = Cwd("testing.cfg")
	} else {
		cfgpath = ""
		userdirflag = flag.String("buser", "", "Defines location of user directory.")
	}
	flag.Parse()

	if *userdirflag != "" {
		baseVardir = *userdirflag
	} else {
		switch runtime.GOOS {
		case "windows":
			baseVardir = fmt.Sprintf("appdata/roaming/%s", project_name)
		// MacOS and Linux are the same.
		default:
			baseVardir = fmt.Sprintf(".%s", project_name)
		}
		usr, e := user.Current()
		if e != nil {
			Log_Error("In bone, failed to retrieve current user.")
			return 1
		}
		baseVardir = path.Join(usr.HomeDir, baseVardir)
	}
	Mkdir(baseVardir)

	if cfgpath == "" {
		cfgpath = Userdir("user.cfg")
	}
	e := Touch(cfgpath)
	if e != nil {
		Log_Error("In bone, failed to create config file.")
		return 1
	}

	c, e := ini.Load(cfgpath)
	if e != nil {
		Log_Error("In bone, failed to load config file.")
		return 1
	}
	Config = &App_Config{
		path: cfgpath,
		data: c,
	}
	return 0
}

func Assert(condition bool, messageAndArgs ...any) {
	if !condition {
		const RED = "\033[91m"
		const RESET = "\033[0m"
		// 1 means get the caller of this function.
		pc, file, line, ok := runtime.Caller(1)
		var message string
		if !ok {
			message = fmt.Sprintf("%sASSERT%s\n", RED, RESET)
		} else {
			// Get the function name.
			funcName := runtime.FuncForPC(pc).Name()
			message = fmt.Sprintf("%s:%d:(%s): %sASSERT%s\n", file, line, funcName, RED, RESET)
		}
		if len(messageAndArgs) > 0 {
			submessage, ok := messageAndArgs[0].(string)
			if !ok {
				panic("In bone during failed assert, passed non-string parameter as message.")
			}
			submessage = fmt.Sprintf(submessage, messageAndArgs[1:]...)
			message = fmt.Sprintf("%s: %s", message, submessage)
		}
		panic(message)
	}
}

// Alpha-to-Pointer.
func Atop(a string) *string {
	return &a
}

type Config_Key = ini.Key

type App_Config struct {
	path string
	data *ini.File
}

var environ_regex = regexp.MustCompile(`\$[A-Z0-9_]+`)

// Return value with environs in format `$ENVIRON` are replaced by found
// variables.
//
// If an environ cannot be found, the block is replaced to `ENVIRON`.
func activate_environs(value string) string {
	matches := environ_regex.FindAllString(value, -1)
	for _, m := range matches {
		environKey, found := strings.CutPrefix(m, "$")
		if !found {
			Log_Error("Incorrect match searching, found '%s'", m)
			continue
		}

		environValue, found := os.LookupEnv(environKey)
		if !found {
			environValue = environKey
		}

		value = strings.Replace(value, m, environValue, 1)
	}
	return value
}

func (cfg *App_Config) Write_String(module string, key string, value string) int {
	// Get or create the section
	section, er := cfg.data.GetSection(module)
	if er != nil {
		// If the section doesn't exist, create it
		section, er = cfg.data.NewSection(module)
		if er != nil {
			Log_Error("In config, cannot create new section, error: %s", er)
			return ERROR
		}
	}

	section.Key(key).SetValue(value)

	er = cfg.data.SaveTo(cfg.path)
	if er != nil {
		Log_Error("Failed to save config, error: %s", er)
		return ERROR
	}

	er = cfg.data.Reload()
	if er != nil {
		Log_Error("Failed to reload config, error: %s", er)
		return ERROR
	}

	return OK
}

func (cfg *App_Config) Get_String(module string, key string, d string) string {
	moduleData, e := cfg.data.GetSection(module)
	if e != nil {
		return d
	}
	valueKey, e := moduleData.GetKey(key)
	if e != nil {
		return d
	}
	valueString := valueKey.String()
	return activate_environs(valueString)
}

func (cfg *App_Config) Get_Bool(module string, key string, d bool) bool {
	moduleData, e := cfg.data.GetSection(module)
	if e != nil {
		return d
	}
	valueKey, e := moduleData.GetKey(key)
	if e != nil {
		return d
	}
	valueBool, e := valueKey.Bool()
	if e != nil {
		Log_Error("For module `%s` and key `%s`, cannot convert value `%s` to bool.", module, key, valueKey.String())
		return d
	}
	return valueBool
}

func (cfg *App_Config) Get_Int(module string, key string, d int) int {
	moduleData, e := cfg.data.GetSection(module)
	if e != nil {
		return d
	}
	valueKey, e := moduleData.GetKey(key)
	if e != nil {
		return d
	}
	valueInt, e := valueKey.Int()
	if e != nil {
		Log_Error("For module `%s` and key `%s`, cannot convert value `%s` to int.", module, key, valueKey.String())
		return d
	}
	return valueInt
}

func (cfg *App_Config) Get_Float(module string, key string, d float64) float64 {
	moduleData, e := cfg.data.GetSection(module)
	if e != nil {
		return d
	}
	valueKey, e := moduleData.GetKey(key)
	if e != nil {
		return d
	}
	value_float, e := valueKey.Float64()
	if e != nil {
		Log_Error("For module `%s` and key `%s`, cannot convert value `%s` to int.", module, key, valueKey.String())
		return d
	}
	return value_float
}

var Config *App_Config

func Testing() bool {
	return testing.Testing()
}

// Needs to be modified at testing to point to proper CWD.
var CwdDepth int = 0
var baseVardir string

func Cwd(pathParts ...string) string {
	// Secure
	path := strings.Join(pathParts, "/")
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ReplaceAll(path, "../", "/")

	cwd, e := os.Getwd()
	if e != nil {
		Log_Error("Cannot retrieve current working directory.")
		cwd = os.TempDir() + fmt.Sprintf("/%s/fakecwd", init_args.Project_Name)
	}
	for i := 0; i < CwdDepth; i++ {
		cwd += "/.."
	}
	return cwd + "/" + path
}

// Access var directory contents.
func Userdir(pathParts ...string) string {
	// Secure
	p := strings.Join(pathParts, "/")
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.ReplaceAll(p, "../", "/")
	return baseVardir + "/" + p
}

func Mkdir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// Touches file on path - creates if not exists, if exists does not truncate.
func Touch(p string) error {
	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0666)
	if e != nil {
		return e
	}
	defer f.Close()
	return nil
}

// Trench - approach to handle string-formatted data to language's objects.
//
// Converting from strings to objects is called "detrenching", from objects to
// strings is called "entrenching".
//
// For example:
//   - "1,10" -> ["1", "10"]
//   - "key=mark,age=20" -> {"key": "mark", "age": "20"}
func Detrench_List(trench string) []string {
	return strings.Split(trench, ",")
}

func File_Exists(path string) bool {
	_, er := os.Stat(path)
	return er == nil
}
