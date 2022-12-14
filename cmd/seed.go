/*
Copyright © 2022 Infinity Bot List
*/
package cmd

import (
	"archive/tar"
	"bytes"
	"compress/lzw"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/InfinityBotList/ibl/helpers"
	"github.com/infinitybotlist/eureka/crypto"

	"github.com/jackc/pgtype"
	"github.com/spf13/cobra"
)

type Meta struct {
	CreatedAt           time.Time `json:"c"`
	Nonce               string    `json:"n"`
	CustomEncryptionKey bool      `json:"e"`
	SeedVer             string    `json:"v"`
	EncryptionSalt      string    `json:"s"`
}

type SourceParsed struct {
	Data  map[string]any
	Table string
}

const seedApiVer = "popplio-e1" // e means encryption protocol version

// seedCmd represents the seed command
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Database Seeding Commands",
	Long:  ``,
}

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Creates a new database seed",
	Long:  `Creates a new database seed`,
	Run: func(cmd *cobra.Command, args []string) {
		cleanup := func() {
			fmt.Println("Cleaning up...")

			// delete all files in work directory
			err := os.RemoveAll("work")

			if err != nil {
				fmt.Println("Error cleaning up:", err)
			}
		}

		var ePass string
		encrypt := cmd.Flag("encrypt").Value.String()
		ePassCmd := cmd.Flag("password").Value.String()

		if encrypt == "true" {
			if ePassCmd == "" {
				ePass = helpers.GetPassword("Enter encryption password: ")
			} else {
				ePass = ePassCmd
			}
		} else {
			ePass = "ibl"
		}

		cleanup()

		// create a work directory
		err := os.Mkdir("work", 0755)

		if err != nil {
			fmt.Println("Error creating work directory:", err)
			cleanup()
			return
		}

		fmt.Println("Creating database backup as schema.sql")

		backupCmd := exec.Command("pg_dump", "-Fc", "--schema-only", "--no-owner", "-d", "infinity", "-f", "work/schema.sql")

		backupCmd.Env = helpers.GetEnv()

		err = backupCmd.Run()

		if err != nil {
			fmt.Println(err)
			cleanup()
			return
		}

		fmt.Println("Copying seed sample data")

		type SeedSource struct {
			Table  string
			Column string
			Value  any
			Desc   string
		}

		var seedDataSrc = []SeedSource{
			{
				Table:  "users",
				Column: "user_id",
				Value:  "563808552288780322",
				Desc:   "Rootsprings User Info",
			},
			{
				Table:  "users",
				Column: "user_id",
				Value:  "728871946456137770",
				Desc:   "Burgerkings User Info",
			},
			{
				Table:  "users",
				Column: "user_id",
				Value:  "510065483693817867",
				Desc:   "Toxic Devs User Info",
			},
			{
				Table:  "bots",
				Column: "bot_id",
				Value:  "1019662370278228028",
				Desc:   "Reedwhisker Bot Info",
			},
			{
				Table:  "bots",
				Column: "bot_id",
				Value:  "815553000470478850",
				Desc:   "IBL Bot Bot Info [for reviews]",
			},
			{
				Table:  "reviews",
				Column: "bot_id",
				Value:  "815553000470478850",
				Desc:   "IBL Bot Review",
			},
			{
				Table:  "votes",
				Column: "bot_id",
				Value:  "1019662370278228028",
				Desc:   "Reedwhisker Votes",
			},
		}

		pool, err := helpers.GetPool()

		if err != nil {
			fmt.Println("Pool connection failed:", err)
			cleanup()
			return
		}

		var parsed = []SourceParsed{}

		for _, source := range seedDataSrc {
			var jsonRow pgtype.JSON
			err := pool.QueryRow(context.Background(), "SELECT row_to_json("+source.Table+") FROM "+source.Table+" WHERE "+source.Column+" = $1", source.Value).Scan(&jsonRow)

			if err != nil {
				fmt.Println("Failed to get data for", source.Desc, err)
				cleanup()
				return
			}

			var data map[string]any
			err = jsonRow.AssignTo(&data)

			if err != nil {
				fmt.Println("Failed to parse data for", source.Desc, err)
				cleanup()
				return
			}

			// Strip tokens from data
			var parsedData = make(map[string]any)
			for k, v := range data {
				if k == "webhook" {
					parsedData[k] = "https://testhook.xyz"
				} else if strings.Contains(k, "token") || strings.Contains(k, "web") {
					parsedData[k] = crypto.RandString(128)
				} else if strings.Contains(k, "unique") {
					parsedData[k] = []any{}
				} else {
					parsedData[k] = v
				}
			}

			parsed = append(parsed, SourceParsed{Data: data, Table: source.Table})
		}

		// Create sample.json buffer
		sampleBuf := bytes.NewBuffer([]byte{})

		// Write sample data to buffer
		enc := json.NewEncoder(sampleBuf)

		err = enc.Encode(parsed)

		if err != nil {
			fmt.Println("Failed to write sample data:", err)
			cleanup()
			return
		}

		// Encrypt sample data
		salt := crypto.RandString(8)
		ePassHashed := helpers.GenKey(ePass, salt)
		sampleBufB, err := helpers.Encrypt([]byte(ePassHashed), sampleBuf.Bytes())

		if err != nil {
			fmt.Println("Failed to encrypt sample data:", err)
			cleanup()
			return
		}

		sampleBuf = bytes.NewBuffer(sampleBufB)

		// Write metadata to buffer
		mdBuf := bytes.NewBuffer([]byte{})

		// Write metadata to md file
		metadata := Meta{
			CreatedAt:           time.Now(),
			Nonce:               crypto.RandString(32),
			CustomEncryptionKey: encrypt == "true",
			SeedVer:             seedApiVer,
			EncryptionSalt:      salt,
		}

		enc = json.NewEncoder(mdBuf)

		err = enc.Encode(metadata)

		if err != nil {
			fmt.Println("Failed to write metadata:", err)
			cleanup()
			return
		}

		// Create a tar file as a io.Writer, NOT a file
		tarFile := bytes.NewBuffer([]byte{})

		if err != nil {
			fmt.Println("Failed to create tar file:", err)
			cleanup()
			return
		}

		tarWriter := tar.NewWriter(tarFile)

		// Write sample buf to tar file
		err = helpers.TarAddBuf(tarWriter, sampleBuf, "data")

		if err != nil {
			fmt.Println("Failed to write sample data to tar file:", err)
			cleanup()
			return
		}

		// Write metadata buf to tar file
		err = helpers.TarAddBuf(tarWriter, mdBuf, "meta")

		if err != nil {
			fmt.Println("Failed to write metadata to tar file:", err)
			cleanup()
			return
		}

		// Write schema to tar file
		schemaFile, err := os.Open("work/schema.sql")

		if err != nil {
			fmt.Println("Failed to open schema file:", err)
			cleanup()
			return
		}

		// -- convert to bytes.Buffer
		schemaBuf := bytes.NewBuffer([]byte{})

		_, err = schemaBuf.ReadFrom(schemaFile)

		if err != nil {
			fmt.Println("Failed to read schema file:", err)
			cleanup()
			return
		}

		schemaBufB, err := helpers.Encrypt([]byte(ePassHashed), schemaBuf.Bytes())

		if err != nil {
			fmt.Println("Failed to encrypt schema data:", err)
			cleanup()
			return
		}

		schemaBuf = bytes.NewBuffer(schemaBufB)

		err = helpers.TarAddBuf(tarWriter, schemaBuf, "schema")

		if err != nil {
			fmt.Println("Failed to write schema to tar file:", err)
			cleanup()
			return
		}

		// Close tar file
		tarWriter.Close()

		compressed, err := os.Create("seed.iblseed")

		if err != nil {
			fmt.Println("Failed to create compressed file:", err)
			cleanup()
			return
		}

		defer compressed.Close()

		// Compress
		w := lzw.NewWriter(compressed, lzw.LSB, 8)

		_, err = io.Copy(w, tarFile)

		if err != nil {
			fmt.Println("Failed to compress file:", err)
			cleanup()
			return
		}

		w.Close()
		cleanup()

		// Try to find /iblseeds folder (devel assets server)
		_, err = os.Stat("/iblseeds")

		if err == nil {
			fmt.Println("Mpving seed to found folder /iblseeds")
			err = os.Rename("seed.iblseed", "/iblseeds/seed.iblseed")

			if err != nil {
				fmt.Println("Failed to copy seed to devel assets server:", err)
			}
		}
	},
}

var applyCmd = &cobra.Command{
	Use:     "apply FILENAME",
	Example: "apply latest",
	Short:   "Apply a seed to the database. You must specify either 'latest' or the path to a seed file.",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var seedf *os.File
		cleanup := func() {
			fmt.Println("Cleaning up...")

			if seedf != nil {
				seedf.Close()
			}

			// delete all files in work directory
			err := os.RemoveAll("work")

			if err != nil {
				fmt.Println("Error cleaning up:", err)
			}
		}

		cleanup()

		// create a work directory
		err := os.Mkdir("work", 0755)

		if err != nil {
			fmt.Println("Error creating work directory", err)
			cleanup()
			return
		}

		// Check args as to which file to use
		seedFile := args[0]

		assetsUrl := helpers.GetAssetsURL()

		if seedFile == "latest" {
			// Download seedfile with progress bar
			data, err := helpers.DownloadFileWithProgress(assetsUrl + "/seed.iblseed?n=" + crypto.RandString(12))

			if err != nil {
				fmt.Println("Failed to download seed file:", err)
				cleanup()
				return
			}

			// Write seedfile to disk as temp.iblseed
			f, err := os.Create("work/temp.iblseed")

			if err != nil {
				fmt.Println("Failed to create temp file:", err)
				cleanup()
				return
			}

			defer f.Close()

			_, err = f.Write(data)

			if err != nil {
				fmt.Println("Failed to download seed file:", err)
				cleanup()
				return
			}

			seedFile = "work/temp.iblseed"
		}

		// Open seed file
		seedf, err = os.Open(seedFile)

		if err != nil {
			fmt.Println("Failed to open seed file:", err)
			cleanup()
			return
		}

		// Extract seed file using lzw to buffer
		tarBuf := bytes.NewBuffer([]byte{})
		r := lzw.NewReader(seedf, lzw.LSB, 8)

		_, err = io.Copy(tarBuf, r)

		if err != nil {
			fmt.Println("Failed to decompress seed file:", err)
			cleanup()
			return
		}

		// Get size of decompressed file
		tarSize := tarBuf.Len()

		fmt.Println("Decompressed size: ", tarSize, "bytes")

		// Extract tar file to map of buffers
		tarReader := tar.NewReader(tarBuf)

		files := make(map[string]*bytes.Buffer)

		for {
			// Read next file from tar header
			header, err := tarReader.Next()

			if err == io.EOF {
				break
			}

			if err != nil {
				fmt.Println("Failed to read tar file:", err)
				cleanup()
				return
			}

			// Read file into buffer
			buf := bytes.NewBuffer([]byte{})

			_, err = io.Copy(buf, tarReader)

			if err != nil {
				fmt.Println("Failed to read tar file:", err)
				cleanup()
				return
			}

			// Save file to map
			files[header.Name] = buf
		}

		fmt.Println("Got map keys:", helpers.MapKeys(files))

		// Extract out meta
		mdBuf, ok := files["meta"]

		if !ok {
			fmt.Println("Seed file is corrupt [no meta]")
			cleanup()
			return
		}

		var md Meta

		err = json.Unmarshal(mdBuf.Bytes(), &md)

		if err != nil {
			fmt.Println("Failed to unmarshal meta:", err)
			cleanup()
			return
		}

		if md.SeedVer != seedApiVer {
			fmt.Println("Seed file is of an invalid version [version is", md.SeedVer, "but expected", seedApiVer, "]")
			return
		}

		// Now finally extract out seed data
		seedBuf, ok := files["data"]

		if !ok {
			fmt.Println("Seed file is corrupt [no data]")
			cleanup()
			return
		}

		// Check if using custom encryption
		var ePass = "ibl"
		if md.CustomEncryptionKey {
			passCmd := cmd.Flag("password").Value.String()

			if passCmd == "" {
				ePass = helpers.GetPassword("This seed is password protected. Please enter the passphrase: ")
			} else {
				ePass = passCmd
			}
		}

		// Decrypt seed data
		ePassHashed := helpers.GenKey(ePass, md.EncryptionSalt)
		seedData, err := helpers.Decrypt([]byte(ePassHashed), seedBuf.Bytes())

		if err != nil {
			fmt.Println("Failed to decrypt seed data:", err)
			cleanup()
			return
		}

		seedBuf = bytes.NewBuffer(seedData)

		// Unpack schema to temp file
		schemaBuf, ok := files["schema"]

		if !ok {
			fmt.Println("Seed file is corrupt [no schema]")
			cleanup()
			return
		}

		schemaData, err := helpers.Decrypt([]byte(ePassHashed), schemaBuf.Bytes())

		if err != nil {
			fmt.Println("Failed to decrypt schema data:", err)
			cleanup()
			return
		}

		schemaBuf = bytes.NewBuffer(schemaData)

		schemaFile, err := os.Create("work/temp.sql")

		if err != nil {
			fmt.Println("Failed to create temp file:", err)
			cleanup()
			return
		}

		defer schemaFile.Close()

		_, err = schemaFile.Write(schemaBuf.Bytes())

		if err != nil {
			fmt.Println("Failed to write temp file:", err)
			cleanup()
			return
		}

		// Ensure PGDATABASE is NOT set
		os.Unsetenv("PGDATABASE")

		pool, err := helpers.GetPoolNoUrl()

		if err != nil {
			fmt.Println("Failed to acquire database pool:", err)
			cleanup()
			return
		}

		// Check if a infinity database already exists
		var exists bool

		err = pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = 'infinity')").Scan(&exists)

		if err != nil {
			fmt.Println("Failed to check if infinity database exists:", err)
			cleanup()
			return
		}

		if exists {
			// Check seed_info table for nonce
			iblPool, err := helpers.GetPool()

			if err != nil {
				fmt.Println("Failed to acquire iblPool:", err, "Ignoring...")
			} else {

				var nonce string

				err = iblPool.QueryRow(context.Background(), "SELECT nonce FROM seed_info").Scan(&nonce)

				if err != nil {
					fmt.Println("Failed to check seed_info table:", err, ". Ignoring...")
				} else {
					if nonce == md.Nonce {
						fmt.Println("You are on the latest seed already!")
						cleanup()
						return
					}
				}
			}

			iblPool.Close()
		}

		// Create role root
		pool.Exec(context.Background(), "CREATE ROLE postgres")
		pool.Exec(context.Background(), "CREATE ROLE root")

		pool.Exec(context.Background(), "DROP DATABASE infinity")

		pool.Exec(context.Background(), "CREATE DATABASE infinity")

		fmt.Println("Restoring database backup")

		// Use pg_restore to restore seedman.tmp
		restoreCmd := exec.Command("pg_restore", "-d", "infinity", "-h", "localhost", "-p", "5432", "work/temp.sql")

		restoreCmd.Stdout = os.Stdout
		restoreCmd.Stderr = os.Stderr

		outCode := restoreCmd.Run()

		if outCode != nil {
			fmt.Println("Failed to restore database backup with error:", outCode)
			cleanup()
			return
		}

		if !restoreCmd.ProcessState.Success() {
			fmt.Println("Failed to restore database backup with unknown error")
			cleanup()
			return
		}

		os.Setenv("PGDATABASE", "infinity")

		pool, err = helpers.GetPool()

		if err != nil {
			fmt.Println("Failed to acquire database pool for newly created database:", err)
			cleanup()
			return
		}

		_, err = pool.Exec(context.Background(), "CREATE TABLE seed_info (nonce TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)")

		if err != nil {
			fmt.Println("Failed to create seed_info table:", err)
			cleanup()
			return
		}

		_, err = pool.Exec(context.Background(), "INSERT INTO seed_info (nonce, created_at) VALUES ($1, $2)", md.Nonce, md.CreatedAt)

		if err != nil {
			fmt.Println("Failed to insert seed info:", err)
			cleanup()
			return
		}

		var seed []SourceParsed

		err = json.Unmarshal(seedBuf.Bytes(), &seed)

		if err != nil {
			fmt.Println("Failed to unmarshal seed:", err)
			cleanup()
			return
		}

		// Loop over seed data and insert into db
		for _, s := range seed {
			var i int = 1
			var args []any
			var keys []string
			var sqlArgs []string

			// Loop over all map props
			for k, v := range s.Data {
				keys = append(keys, k)
				args = append(args, v)
				sqlArgs = append(sqlArgs, "$"+strconv.Itoa(i))
				i++
			}

			// Create sql string
			fmt.Println(s.Table)
			sql := "INSERT INTO " + s.Table + " (" + strings.Join(keys, ", ") + ") VALUES (" + strings.Join(sqlArgs, ", ") + ")"

			fmt.Println(sql, args)

			_, err := pool.Exec(context.Background(), sql, args...)

			if err != nil {
				fmt.Println("Failed to insert seed data:", err)
				cleanup()
				return
			}
		}

		cleanup()
	},
}

func init() {
	// seed new
	seedCmd.AddCommand(newCmd)

	// seed apply
	seedCmd.AddCommand(applyCmd)

	rootCmd.AddCommand(seedCmd)

	newCmd.Flags().BoolP("encrypt", "e", false, "Encrypt seed file with custom password")
	newCmd.Flags().StringP("password", "p", "", "Password to encrypt seed file with. Otherwise interactive prompt will be used")

	applyCmd.Flags().StringP("password", "p", "", "Password to decrypt seed file with. Otherwise interactive prompt will be used if required")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// seedCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// seedCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
