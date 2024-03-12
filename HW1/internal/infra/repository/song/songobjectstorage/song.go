package songobjectstorage

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/model"
	"github.com/SepehrNoey/Cloud-Computing-Homeworks/internal/domain/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Repository struct {
	accessKey  string
	secretKey  string
	endpoint   string
	bucketName string
}

func New(accessKey string, secretKey string, endpoint string, bucketName string) *Repository {
	return &Repository{
		accessKey:  accessKey,
		secretKey:  secretKey,
		endpoint:   endpoint,
		bucketName: bucketName,
	}
}

func (r *Repository) Create(ctx context.Context, song model.Song) error {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(r.accessKey, r.secretKey, ""),
		Region:      aws.String("default"),
		Endpoint:    aws.String(r.endpoint),
	})
	if err != nil {
		fmt.Printf("error in making session to object storage for upload: %s\n", err.Error())
		return err
	}
	fmt.Println("session created")
	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(strconv.Itoa(song.ReqID)),
		Body:   bytes.NewReader(util.Serialize(song)),
	})
	if err != nil {
		fmt.Printf("error in creating uploader: %s\n", err.Error())
		return err
	}

	fmt.Println("uploaded successfully")
	return nil
}

func (r *Repository) Get(ctx context.Context, reqID int) *model.Song {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(r.accessKey, r.secretKey, ""),
		Region:      aws.String("default"),
		Endpoint:    aws.String(r.endpoint),
	})
	if err != nil {
		fmt.Printf("error in making session to object storage for download: %s\n", err.Error())
		return nil
	}

	downloader := s3manager.NewDownloader(sess)
	buff := aws.NewWriteAtBuffer([]byte{})

	_, err = downloader.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(strconv.Itoa(reqID)),
	})
	if err != nil {
		fmt.Printf("error in downloading object from object storage: %s\n", err.Error())
		return nil
	}

	var song model.Song
	util.Deserialize(buff.Bytes(), &song)
	return &song
}
