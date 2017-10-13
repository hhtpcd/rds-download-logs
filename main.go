package main // import "github.com/hhtpcd/rds-download-logs"

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	flag "github.com/jessevdk/go-flags"
)

type Opts struct {
	DatabaseInstance string `short:"d" long:"database" description:"Identifier of the database instance"`
	Region           string `short:"r" long:"region" description:"Amazon region" default:"eu-west-1"`
	LogOutput        string `short:"o" long:"output" description:"Location for saving the log" default:"."`
	Follow           bool   `short:"f" long:"follow" description:"Follow a log named in -l flag"`
	LogName          string `short:"l" long:"log" description:"Specify a log when used in conjunction with -s/-l"`
	Download         bool   `short:"s" long:"save" description:"Choose to download a logfile"`
	PrintLogs        bool   `short:"p" long:"print" description:"Print log file names to stdout"`
}

type app struct {
	Options *Opts
	RDS     *rds.RDS
	Cfg     *aws.Config
	S3      *s3.S3
}

func main() {
	var opts Opts
	fp := flag.NewParser(&opts, flag.Default)
	if _, err := fp.Parse(); err != nil {
		if flagsErr, ok := err.(*flag.Error); ok && flagsErr.Type == flag.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	a := &app{
		Options: &opts,
		RDS: rds.New(session.New(), &aws.Config{
			Region: aws.String(opts.Region),
		}),
		S3: s3.New(session.New(), &aws.Config{
			Credentials: credentials.NewEnvCredentials(),
			Region:      aws.String(opts.Region),
		}),
	}

	l, err := a.getLogNames()
	if err != nil {
		panic(err)
	}

	if opts.LogName != "" {
		if opts.Follow {
			err := a.followLog(opts.LogName)
			if err != nil {
				panic(err)
			}
		}

		if opts.Download && opts.DatabaseInstance != "" {
			path, err := a.downloadLogFile(opts.LogName)
			if err != nil {
				panic(err)
			}

			fmt.Println(path)
			os.Exit(0)
		}

		if a.Options.PrintLogs {
			for _, k := range l.DescribeDBLogFiles {
				fmt.Println(*k.LogFileName)
			}
			os.Exit(0)
		}
	}
}

// getLogNames returns an object that contains a list of log files
// for the database instance and log name from the application options
func (a *app) getLogNames() (*rds.DescribeDBLogFilesOutput, error) {
	input := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(a.Options.DatabaseInstance),
		FilenameContains:     aws.String(a.Options.LogName),
	}

	return a.RDS.DescribeDBLogFiles(input)
}

// getRecentEntries will download a single line of a log portion
// from the start position and return the RDS log output
func (a *app) getRecentEntries(sPos string, logName string) (*rds.DownloadDBLogFilePortionOutput, error) {
	params := &rds.DownloadDBLogFilePortionInput{
		DBInstanceIdentifier: aws.String(a.Options.DatabaseInstance),
		LogFileName:          aws.String(logName),
		Marker:               aws.String(sPos),
		NumberOfLines:        aws.Int64(1),
	}

	return a.RDS.DownloadDBLogFilePortion(params)
}

func (a *app) followLog(logName string) error {
	sPos := "0"

	for {
		resp, err := a.getRecentEntries(sPos, logName)
		if err != nil {
			if _, ok := err.(awserr.Error); ok {
				return err
			}
		}

		if resp.LogFileData != nil {
			fmt.Print(*resp.LogFileData)
		}

		sPos = *resp.Marker

		if !*resp.AdditionalDataPending {
			fmt.Printf("No data pending. Sleeping %d sec \n", 10)
			time.Sleep(time.Duration(15) * time.Second)
		}
	}
}

// downloadLogFile will download a full RDS log at once from the AWS
// REST API Endpoint that is not available through the Go SDK.
// It will return an absolute string path to the file.
func (a *app) downloadLogFile(logName string) (string, error) {
	client := &http.Client{}

	cfg := new(aws.Config)
	cfg.Credentials = credentials.NewEnvCredentials()
	signer := v4.NewSigner(cfg.Credentials)

	host := fmt.Sprintf("https://rds.%s.amazonaws.com", a.Options.Region)

	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	req.URL.Path = fmt.Sprintf("/v13/downloadCompleteLogFile/%s/%s", a.Options.DatabaseInstance, logName)

	_, err = signer.Sign(req, nil, "rds", a.Options.Region, time.Unix(time.Now().Unix(), 0))
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return "", err
	}

	fmt.Printf("Download request completed with status: %d > %s \n", resp.StatusCode, req.URL.Path)

	defer resp.Body.Close()

	// Create the file
	fileName := strings.Split(logName, "/")
	outputPath := filepath.Join(a.Options.LogOutput, fileName[1])
	out, err := os.Create(outputPath)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer out.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	return outputPath, nil
}

