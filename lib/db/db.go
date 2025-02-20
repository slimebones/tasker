package db

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"tasker/lib/bone"
	"tasker/lib/dog"

	_ "github.com/glebarez/go-sqlite"
	"github.com/jmoiron/sqlx"
)

type Tx = sqlx.Tx

var connection *sqlx.DB

const VERSION_START_QUERY = `
	CREATE TABLE IF NOT EXISTS db_version (
		version INTEGER NOT NULL
	);
	INSERT INTO db_version (version) VALUES (1);
`

const VERSION_GET_QUERY = `SELECT version FROM db_version LIMIT 1`

const ()

var driver string
var addr string
var maxOpen int
var maxIdle int

const (
	Ok = iota
	Error
	Empty_Driver_Error
	Version_Fetch_Error
	Empty_Addr_Error
	Connection_Error
	Dir_Read_Error
)

func getVersion() (int, int) {
	var version int
	tx := Begin()
	defer tx.Rollback()
	e := tx.Get(&version, VERSION_GET_QUERY)
	if e != nil {
		return 0, Version_Fetch_Error
	}
	if version < 1 {
		bone.Log_Error("In db, version cannot be `%d`", version)
		return 0, 1
	}
	return version, 0
}

func Begin() *Tx {
	return connection.MustBegin()
}

func getSortedMigrations() ([]string, int) {
	files, er := os.ReadDir(migrationsDir)
	if er != nil {
		return nil, Dir_Read_Error
	}
	filenames := []string{}
	for _, file := range files {
		filename := file.Name()
		if strings.HasSuffix(filename, ".sql") {
			filenames = append(filenames, filename)
		}
	}
	sliceError := false
	sort.Slice(filenames, func(i, j int) bool {
		date0, er := strconv.Atoi(strings.Split(filenames[i], "_")[0])
		if er != nil {
			sliceError = true
			return false
		}
		date1, er := strconv.Atoi(strings.Split(filenames[j], "_")[0])
		if er != nil {
			sliceError = true
			return false
		}
		return date0 < date1
	})
	if sliceError {
		bone.Log_Error("In db, slicing error")
		return nil, 1
	}
	return filenames, 0
}

func startVersion() {
	tx := Begin()
	_, e := tx.Exec(VERSION_START_QUERY)
	if e != nil {
		panic(fmt.Sprintf("In db, cannot start version for the database, error: %s", e.Error()))
	}
	e = tx.Commit()
	if e != nil {
		panic(fmt.Sprintf("In db, cannot start version for the database, error: %s", e.Error()))
	}
}

func getMigrationsFrom(version int, migrations []string) []string {
	return migrations[version-1:]
}

// v1 for empty database, otherwise current database version + 1.
func getNextVersion() int {
	currentVersion, e := getVersion()
	if e == Version_Fetch_Error {
		currentVersion = -1
	} else if e > 0 {
		return e
	}

	nextVersion := currentVersion + 1
	if nextVersion == 0 {
		startVersion()
		nextVersion = 1
	}
	return nextVersion
}

func setVersion(tx *Tx, version int) int {
	_, er := tx.Exec("UPDATE db_version SET version = $1", version)
	if er != nil {
		return 1
	}
	return 0
}

var migrationsDir string

// Note: migration down is not yet supported.
func sync() int {
	migrationsDir = bone.Cwd("migrations")
	nextVersion := getNextVersion()

	migrations, e := getSortedMigrations()
	if e > 0 {
		return e
	}
	migrations = getMigrationsFrom(nextVersion, migrations)
	return applyMigrations(migrations)
}

func applyMigrations(migrations []string) int {
	dogvars := map[string]string{
		fmt.Sprintf("DRIVER_%s", strings.ToUpper(driver)): "",
	}
	for _, m := range migrations {
		version, er := strconv.Atoi(strings.Split(m, "_")[0])
		if er != nil {
			bone.Log_Error("In db, encountered wrong migration name `%s`.", m)
			return 1
		}

		bone.Log("In db, sync to version %d.", version)

		p := migrationsDir + "/" + m
		body, ok := dog.ProcessFile(p, dogvars)
		if !ok {
			bone.Log_Error("In db, failed to process file `%s` by dog.", p)
			return 1
		}

		tx := Begin()
		query := *body
		_, er = tx.Exec(query)
		if er != nil {
			bone.Log_Error("In db, failed to execute query of file `%s`, error: %s", p, er)
			tx.Rollback()
			return 1
		}

		e := setVersion(tx, version)
		if e > 0 {
			bone.Log_Error("Error setting version")
			tx.Rollback()
			return e
		}

		er = tx.Commit()
		if er != nil {
			tx.Rollback()
			bone.Log_Error("In db, failed to commit query of file `%s`, error: %s", p, er)
			return 1
		}
	}
	return 0
}

// Once we connect the database, we *always* sync it to the latest version
// possible. Latest version is defined by the `db/migrations` directory.
//
// If we cannot sync, we cannot start working with database. In such case we
// panic.
func Init(should_sync bool) int {
	driver = bone.Config.GetString("db", "driver", "sqlite")
	addr = bone.Config.GetString("db", "addr", ":memory:")
	maxOpen = bone.Config.GetInt("db", "max_open", 0)
	maxIdle = bone.Config.GetInt("db", "max_idle", 2)

	if driver == "" {
		bone.Log_Error("In db, empty driver")
		return Empty_Driver_Error
	}
	if addr == "" {
		bone.Log_Error("In db, empty addr")
		return Empty_Addr_Error
	}

	_db, er := sqlx.Connect(
		driver,
		addr,
	)
	if er != nil {
		bone.Log_Error("In db, connection error: %s", er)
		return Connection_Error
	}

	connection = _db
	connection.SetMaxOpenConns(maxOpen)
	connection.SetMaxIdleConns(maxIdle)

	if driver == "sqlite" {
		// Enable FK enforcement by default for SQLite
		// https://stackoverflow.com/a/58611190/14748231
		connection.MustExec("PRAGMA foreign_keys = 1")
	}

	if should_sync {
		e := sync()
		if e > 0 {
			bone.Log_Error("In db, sync issue")
			return e
		}
	}

	return 0
}

func Deinit() {
	if connection != nil {
		connection.Close()
		connection = nil
	}
}
