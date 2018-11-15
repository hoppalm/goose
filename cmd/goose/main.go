package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/pressly/goose"

	// Init DB drivers.
	"github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/ziutek/mymysql/godrv"
)

var (
	flags = flag.NewFlagSet("goose", flag.ExitOnError)
	dir   = flags.String("dir", ".", "directory with migration files")

	tlsName       = flags.String("tlsname", "", "name of the TLS cert")
	caCert        = flags.String("cacert", "", "CA Cert file")
	clientCert    = flags.String("clientcert", "", "Client Cert file")
	clientKey     = flags.String("clientkey", "", "Client Key file")
	useClientCert = flags.Bool("useclientcert", false, "Use client cert to connect")
)

func main() {
	flags.Usage = usage
	flags.Parse(os.Args[1:])

	args := flags.Args()
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		flags.Usage()
		return
	}

	switch args[0] {
	case "create":
		if err := goose.Run("create", nil, *dir, args[1:]...); err != nil {
			log.Fatalf("goose run: %v", err)
		}
		return
	case "fix":
		if err := goose.Run("fix", nil, *dir); err != nil {
			log.Fatalf("goose run: %v", err)
		}
		return
	}

	if len(args) < 3 {
		flags.Usage()
		return
	}

	if *caCert != "" || *clientCert != "" || *clientKey != "" || *tlsName != "" {
		if args[0] != "mysql" {
			log.Fatal("cacert, clientcert, clientkey flags should only be set if the driver is mysql")
		}

		if (*caCert == "" || *tlsName == "") || ((*clientCert == "" || *clientKey == "") && *useClientCert) {
			log.Fatal("cacert needs to always be set and client cert/key needs to be set if using clientcert")
		}

		setupTLS()
	}

	driver, dbstring, command := args[0], args[1], args[2]

	if err := goose.SetDialect(driver); err != nil {
		log.Fatal(err)
	}

	switch driver {
	case "redshift":
		driver = "postgres"
	case "tidb":
		driver = "mysql"
	}

	switch dbstring {
	case "":
		log.Fatalf("-dbstring=%q not supported\n", dbstring)
	default:
	}

	db, err := sql.Open(driver, dbstring)
	if err != nil {
		log.Fatalf("-dbstring=%q: %v\n", dbstring, err)
	}

	arguments := []string{}
	if len(args) > 3 {
		arguments = append(arguments, args[3:]...)
	}

	if err := goose.Run(command, db, *dir, arguments...); err != nil {
		log.Fatalf("goose run: %v", err)
	}
}

func usage() {
	log.Print(usagePrefix)
	flags.PrintDefaults()
	log.Print(usageCommands)
}

var (
	usagePrefix = `Usage: goose [OPTIONS] DRIVER DBSTRING COMMAND

Drivers:
    postgres
    mysql
    sqlite3
    redshift

Examples:
    goose sqlite3 ./foo.db status
    goose sqlite3 ./foo.db create init sql
    goose sqlite3 ./foo.db create add_some_column sql
    goose sqlite3 ./foo.db create fetch_user_data go
    goose sqlite3 ./foo.db up

    goose postgres "user=postgres dbname=postgres sslmode=disable" status
    goose mysql "user:password@/dbname?parseTime=true" status
    goose redshift "postgres://user:password@qwerty.us-east-1.redshift.amazonaws.com:5439/db" status
    goose tidb "user:password@/dbname?parseTime=true" status

Options:
`

	usageCommands = `
Commands:
    up                   Migrate the DB to the most recent version available
    up-to VERSION        Migrate the DB to a specific VERSION
    down                 Roll back the version by 1
    down-to VERSION      Roll back to a specific VERSION
    redo                 Re-run the latest migration
    reset                Roll back all migrations
    status               Dump the migration status for the current DB
    version              Print the current version of the database
    create NAME [sql|go] Creates new migration file with the current timestamp
    fix                  Apply sequential ordering to migrations
`
)

func setupTLS() {
	// Load CA into cert pool
	pemEncryptedCACert, err := ioutil.ReadFile(*caCert)
	if err != nil {
		log.Fatal(err)
	}

	rootCertPool := x509.NewCertPool()
	if ok := rootCertPool.AppendCertsFromPEM(pemEncryptedCACert); !ok {
		log.Fatal(err)
	}
	clientCerts := make([]tls.Certificate, 0, 1)
	if *useClientCert {
		// Load cert/key into certificate
		clientCert, err := tls.LoadX509KeyPair(*clientCert, *clientKey)
		if err != nil {
			log.Fatal(err)
		}
		clientCerts = append(clientCerts, clientCert)
	}
	// Create and register tls Config for use by mysql
	mysql.RegisterTLSConfig(*tlsName, &tls.Config{
		RootCAs:      rootCertPool,
		Certificates: clientCerts,
	})

	log.Println("tls enabled")
}
