package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/Masterminds/structable"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

const (
	DB_FLAVOR = "sqlite3"
	DB_SOURCE = "database.db"
)

func createDB(dataDir string, dataSrc string) {
	log.Println("Creating database file")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		log.Fatal(err.Error())
	}

	file, err := os.Create(dataSrc)
	if err != nil {
		log.Fatal(err.Error())
	}
	file.Close()
	log.Println("Database file created")
}

func initDB(dataType string, dataSrc string) {
	con, _ := sql.Open(dataType, dataSrc)
	defer con.Close()

	aStatements := map[string]string{
		TB_NAME_AGENTS:  TB_SCHEMA_AGENTS,
		TB_NAME_JOBS:    TB_SCHEMA_JOBS,
		TB_NAME_CRASHES: TB_SCHEMA_CRASHES,
		TB_NAME_STATS:   TB_SCHEMA_STATS,
		TB_NAME_USERS:   TB_SCHEMA_USERS,
	}

	for n, s := range aStatements {
		log.Printf("Creating '%s' table\n", n)
		statement, err := con.Prepare(s)
		if err != nil {
			log.Fatal(err.Error())
		}
		statement.Exec()
		log.Printf("Table '%s' created\n", n)
	}
}

func getDB() squirrel.DBProxyBeginner {
	dataType := DB_FLAVOR
	dataDir := viper.GetString("data.dir")
	dataSrc := filepath.Join(dataDir, DB_SOURCE)

	if !fileExists(dataSrc) {
		createDB(dataDir, dataSrc)
		initDB(dataType, dataSrc)
		initUser()
	}

	con, _ := sql.Open(dataType, dataSrc)
	cache := squirrel.NewStmtCacheProxy(con)

	return cache
}

func listWhere(d structable.Recorder, fn structable.WhereFunc) ([]structable.Recorder, error) {
	var tn string = d.TableName()
	var cols []string = d.Columns(true)
	buf := []structable.Recorder{}

	// Base query
	q := d.Builder().Select(cols...).From(tn)

	// Allow the fn to modify our query
	var err error
	q, err = fn(d, q)
	if err != nil {
		return buf, err
	}

	rows, err := q.Query()
	if err != nil || rows == nil {
		return buf, err
	}
	defer rows.Close()

	v := reflect.Indirect(reflect.ValueOf(d))
	t := v.Type()
	for rows.Next() {
		nv := reflect.New(t)

		// Bind an empty base object. Basically, we fetch the object out of
		// the DbRecorder, and then construct an empty one.
		rec := reflect.New(reflect.Indirect(reflect.ValueOf(d.(*structable.DbRecorder).Interface())).Type())
		nv.Interface().(structable.Recorder).Bind(d.TableName(), rec.Interface())

		s := nv.Interface().(structable.Recorder)
		s.Init(d.DB(), d.Driver())
		dest := s.FieldReferences(true)

		if err := rows.Scan(dest...); err != nil {
			return buf, err
		}

		buf = append(buf, s)
	}

	return buf, rows.Err()
}
