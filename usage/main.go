package main

import (
	"os"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var maxRetries = 100

func buildTypeMap(types *ec2.DescribeInstanceTypesOutput) map[string] int64 {
	typesMap := map[string] int64 {}

	for _, t := range types.InstanceTypes {
		if t.VCpuInfo.DefaultVCpus != nil {
			typesMap[*t.InstanceType + ".vcpus"] = *t.VCpuInfo.DefaultVCpus
		}

		if t.VCpuInfo.DefaultCores != nil {
			typesMap[*t.InstanceType + ".cores"] = *t.VCpuInfo.DefaultCores
		}

		if t.MemoryInfo.SizeInMiB != nil {
			typesMap[*t.InstanceType + ".memory"] = *t.MemoryInfo.SizeInMiB
		}
	}

	return typesMap
}

var stats = map[string] int64 {}

func captureVolumes(region string, result *ec2.DescribeVolumesOutput) {
	for _, volume := range result.Volumes {
		for _, prefix := range []string {"total.", region + "."} {
			stats[prefix + "volumes.size_gib"] = stats[prefix + "volumes.size_gib"] + *volume.Size
			stats[prefix + "volumes.count"] = stats[prefix + "volumes.count"] + 1
		}
	}
}

func getVolumes(svc *ec2.EC2) *ec2.DescribeVolumesOutput {
	result, err := svc.DescribeVolumes(&ec2.DescribeVolumesInput{})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		fmt.Println(result)
	}
	return result
}

func getAddresses(svc *ec2.EC2) *ec2.DescribeAddressesOutput {
	result, err := svc.DescribeAddresses(&ec2.DescribeAddressesInput{})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		fmt.Println(result)
	}
	return result
}

func captureInstances(
	region string,
	result *ec2.DescribeInstancesOutput,
	types *ec2.DescribeInstanceTypesOutput,
	states []*string) {

	tm := buildTypeMap(types)

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			for _, preprefix := range []string {"total.", region + "."} {
				prefix := preprefix + *states[0] + "."
				stats[prefix + "instances"] = stats[prefix + "instances"] + 1
				stats[prefix + "instances." + *instance.InstanceType] = stats[prefix + "instances." + *instance.InstanceType] + 1

				stats[prefix + "vcpus"] = stats[prefix + "vcpus"] + tm[*instance.InstanceType + ".vcpus"]
				stats[prefix + "cores"] = stats[prefix + "cores"] + tm[*instance.InstanceType + ".cores"]
				stats[prefix + "memory_mib"] = stats[prefix + "memory_mib"] + tm[*instance.InstanceType + ".memory"]
			}
		}
	}
}

func getInstances(svc *ec2.EC2, states []*string) *ec2.DescribeInstancesOutput {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: states,
			},
		},
	}

	instances := []*DescribeInstancesOutput{}
	err := svc.DescribeInstances(input,
		func(page *ec2.DescribeInstanceOutput, lastePage bool) bool {
			instances = instances

		}

	)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil
	}
	return instances
}

func main() {
	if os.Getenv("AWS_PROFILE") == "" {
		os.Setenv("AWS_PROFILE", "gpte")
	}

	if os.Getenv("AWS_REGION") == "" {
		os.Setenv("AWS_REGION", "us-east-1")
	}

	sess, _ := session.NewSession(
		&aws.Config{
			Region:     aws.String(os.Getenv("AWS_REGION")),
			MaxRetries: &maxRetries,
		},
	)

	svcGlob := ec2.New(sess)

	types, _ := svcGlob.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{})
	regions, _ := svcGlob.DescribeRegions(&ec2.DescribeRegionsInput{})

	for _, region := range regions.Regions {
		if *region.RegionName != "eu-central-1" {
			continue
		}
		sess, _ := session.NewSession(
			&aws.Config{
				Region:     region.RegionName,
				MaxRetries: &maxRetries,
			},
		)
		svc := ec2.New(sess)

		volumes := getVolumes(svc)
		captureVolumes(*region.RegionName, volumes)

		states := []*string{
			aws.String("running"),
			aws.String("pending"),
		}
		instances := getInstances(svc, states)
		captureInstances(*region.RegionName, instances, types, states)

		states = []*string{
			aws.String("stopped"),
			aws.String("shutting-down"),
			aws.String("stopping"),
		}
		instances = getInstances(svc, states)
		captureInstances(*region.RegionName, instances, types, states)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	enc.Encode(&stats)
}
