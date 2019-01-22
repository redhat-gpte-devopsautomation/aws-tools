package main

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"strings"
)

var svcIam *iam.IAM

func iamInstanceProfileExists(instanceprofileId string) bool {
	v("exists?", instanceprofileId)

	// Skip full ids, test only InstanceProfile names
	//if strings.Contains(instanceprofileId, "arn:aws:iam") {
	//return false
	//}
	if svcIam == nil {
		svcIam = iam.New(sess)
	}

	input := &iam.GetInstanceProfileInput{
		InstanceProfileName: &instanceprofileId,
	}
	_, err := svcIam.GetInstanceProfile(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NoSuchEntity":
				return false
			case "ValidationError":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			logErr.Println(err.Error())
		}
		return false
	} else {
		return true
	}
}

func iamRoleExists(RoleId string) bool {
	v("exists?", RoleId)

	// Skip full ids, test only Role names
	if strings.Contains(RoleId, "arn:aws:iam") {
		return false
	}
	if svcIam == nil {
		svcIam = iam.New(sess)
	}

	input := &iam.GetRoleInput{
		RoleName: &RoleId,
	}
	_, err := svcIam.GetRole(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NoSuchEntity":
				return false
			case "ValidationError":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			logErr.Println(err.Error())
		}
		return false
	} else {
		return true
	}
}
