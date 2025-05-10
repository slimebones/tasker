package db

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"tasker/internal/bone"
	"tasker/internal/dog"

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

const (
	VERSION_FETCHING = 2
)

func get_version(tx *Tx) (int, int) {
	var version int
	e := tx.Get(&version, VERSION_GET_QUERY)
	if e != nil {
		return 0, VERSION_FETCHING
	}
	if version < 1 {
		bone.Log_Error("In db, version cannot be `%d`.", version)
		return 0, 1
	}
	return version, 0
}

// Begin a new transaction.
// **Always** call `defer tx.Rollback()`. If you commit, then rollback, it
// won't hurt in anyway.
func Begin() *Tx {
	return connection.MustBegin()
}

func getSortedMigrations() ([]string, int) {
	files, er := os.ReadDir(migrations_dir)
	if er != nil {
		bone.Log_Error("Cannot read migrations dir: " + migrations_dir)
		return nil, 1
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
		return nil, 1
	}
	return filenames, 0
}

func start_version(tx *Tx) {
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
func get_next_version() int {
	tx := Begin()
	defer tx.Rollback()
	currentVersion, e := get_version(tx)
	if e == VERSION_FETCHING {
		currentVersion = -1
	} else if e > 0 {
		return e
	}

	nextVersion := currentVersion + 1
	if nextVersion == 0 {
		start_version(tx)
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

var migrations_dir string

// Note: migration down is not yet supported.
func sync() int {
	migrations_dir = bone.Cwd("migrations")
	version := get_next_version()
	bone.Log("Database version: %d", version)

	migrations, e := getSortedMigrations()
	if e > 0 {
		return e
	}
	migrations = getMigrationsFrom(version, migrations)
	return apply_migrations(migrations)
}

func apply_migrations(migrations []string) int {
	dogvars := map[string]string{
		fmt.Sprintf("DRIVER_%s", strings.ToUpper(driver)): "",
	}
	for _, m := range migrations {
		version, er := strconv.Atoi(strings.Split(m, "_")[0])
		if er != nil {
			bone.Log_Error("In db, encountered wrong migration name `%s`.", m)
			return 1
		}

		bone.Log("In db, sync to version %d", version)

		p := migrations_dir + "/" + m
		body, ok := dog.ProcessFile(p, dogvars)
		if !ok {
			bone.Log_Error("In db, failed to process file `%s` by dog.", p)
			return 1
		}

		tx := Begin()
		defer tx.Rollback()
		query := *body
		_, er = tx.Exec(query)
		if er != nil {
			bone.Log_Error("In db, failed to execute query of file `%s`, error: %s", p, er)
			return 1
		}

		setVersion(tx, version)

		er = tx.Commit()
		if er != nil {
			bone.Log_Error("In db, failed to commit query of file `%s`, error: %s", p, er)
		}
	}
	return 0
}

var driver string
var addr string
var maxOpen int
var maxIdle int

// Once we connect the database, we *always* sync it to the latest version
// possible. Latest version is defined by the `db/migrations` directory.
//
// If we cannot sync, we cannot start working with database. In such case we
// panic.
func Init() int {
	driver = bone.Config.Get_String("db", "driver", "sqlite")
	addr = bone.Config.Get_String("db", "addr", ":memory:")
	maxOpen = bone.Config.Get_Int("db", "max_open", 0)
	maxIdle = bone.Config.Get_Int("db", "max_idle", 2)

	if driver == "" {
		bone.Log_Error("Empty `db.driver`.")
		return 1
	}
	if addr == "" {
		bone.Log_Error("Empty `db.addr`.")
		return 1
	}

	_db, er := sqlx.Connect(
		driver,
		addr,
	)
	if er != nil {
		return 1
	}

	connection = _db
	connection.SetMaxOpenConns(maxOpen)
	connection.SetMaxIdleConns(maxIdle)

	if driver == "sqlite" {
		// Enable FK enforcement by default for SQLite
		// https://stackoverflow.com/a/58611190/14748231
		connection.MustExec("PRAGMA foreign_keys = 1")
	}

	e := sync()
	if e > 0 {
		return e
	}

	return 0
}

func Deinit() {
	if connection != nil {
		connection.Close()
		connection = nil
	}
}
