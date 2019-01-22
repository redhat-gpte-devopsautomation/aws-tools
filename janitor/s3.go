package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

var svcS3 *s3.S3

func s3BucketExists(bucketId string) bool {
	v("exists?", bucketId)
	if svcS3 == nil {
		svcS3 = s3.New(sess)
	}

	input := &s3.HeadBucketInput{
		Bucket: aws.String(bucketId),
	}
	_, err := svcS3.HeadBucket(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				return false
			case s3.ErrCodeNoSuchBucket:
				return false
			default:
				logErr.Println(aerr.Error())
				return false
			}
		} else {
			logErr.Println(err.Error())
			return false
		}
	}

	return false
}
