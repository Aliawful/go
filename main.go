// Package p contains a Google Cloud Storage Cloud Function.
package p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/yeka/zip"

	"cloud.google.com/go/storage"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	BUCKET_NAME   = "s-cactus"
	BUCKET_FOLDER = "reporting/sik"

	FTP_PASSWORD = "pass"
	FTP_SERVER   = "172.17.0.3"
	FTP_PORT     = "22"
	FTP_USER     = "foo"
	FTP_FOLDER   = "in"

	ZIP_PASS = "P0h0nD4n4"
)

// GCSEvent is the payload of a GCS event. Please refer to the docs for
// additional information regarding GCS events.
type GCSEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

// HelloGCS prints a message when a file is changed in a Cloud Storage bucket.
func Main(ctx context.Context, e GCSEvent) error {
	if isInFolder(BUCKET_FOLDER, fmt.Sprintf("%s", e.Name)) {
		log.Printf("Processing file: %s", e.Name)

		f, err := getFileFromBucket(e.Name)
		if err != nil {
			return err
		}

		log.Printf("Zipping file...")
		if err := zipFile(removeFolderPath(e.Name), f); err != nil {
			log.Printf("Error when zipping file")
			return err
		}

		log.Printf("Sending file to ftp server...")
		if err := sendToFtp(removeFolderPath(e.Name)); err != nil {
			return err
		}

		log.Printf("Done!")
		return nil
	}

	log.Printf("ignoring file: %s", e.Name)
	return nil
}

func getFileFromBucket(obj string) ([]byte, error) {
	ctx := context.Background()

	// Creates a client.
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	rc, err := client.Bucket(BUCKET_NAME).Object(obj).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func sendToFtp(filename string) error {
	config := &ssh.ClientConfig{
		User: FTP_USER,
		Auth: []ssh.AuthMethod{
			ssh.Password(FTP_PASSWORD),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// connect
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", FTP_SERVER, FTP_PORT), config)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create new SFTP client
	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()

	// create destination file
	dstFile, err := client.Create(fmt.Sprintf("%s/%s.zip", FTP_FOLDER, removeCsvExtension(filename)))
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// create source file
	srcFile, err := os.Open(fmt.Sprintf("/tmp/%s.zip", filename))
	if err != nil {
		return err
	}

	// copy source file to destination file
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func zipFile(filename string, data []byte) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	zipFile, err := os.OpenFile(fmt.Sprintf("/tmp/%s.zip", filename), flags, 0644)
	if err != nil {
		log.Printf("Failed to open zip for writing: %s", err)
		return err
	}
	defer zipFile.Close()

	zipw := zip.NewWriter(zipFile)
	defer zipw.Close()

	reader := bytes.NewReader(data)

	wr, err := zipw.Encrypt(fmt.Sprintf("%s", filename), ZIP_PASS, zip.StandardEncryption)
	if err != nil {
		return err
	}

	if _, err := reader.WriteTo(wr); err != nil {
		log.Printf("Failed to write %s to zip: %s", filename, err)
		return err
	}

	return nil
}

func isInFolder(folder string, name string) bool {
	return strings.HasPrefix(name, folder)
}

func removeFolderPath(name string) string {
	return strings.TrimLeft(name, BUCKET_FOLDER)
}

func removeCsvExtension(name string) string {
	return strings.TrimRight(name, ".csv")
}
