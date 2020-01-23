package main

import (
	"os"
	"encoding/json"
	"fmt"
	"log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var maxRetries = 100
var stats = map[string] int64 {}

// Logging
var logErr *log.Logger
var logOut *log.Logger


func buildTypeMap(types []*ec2.InstanceTypeInfo) map[string] int64 {
	typesMap := map[string] int64 {}

	for _, t := range types {
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

func captureVolumes(region string, result []*ec2.Volume) {
	for _, volume := range result {
		for _, prefix := range []string {"total.", region + "."} {
			stats[prefix + "volumes.size_gib"] = stats[prefix + "volumes.size_gib"] + *volume.Size
			stats[prefix + "volumes.count"] = stats[prefix + "volumes.count"] + 1
		}
	}
}

func getVolumes(svc *ec2.EC2) []*ec2.Volume {
	volumes := []*ec2.Volume{}

	err := svc.DescribeVolumesPages(&ec2.DescribeVolumesInput{},
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			volumes = append(volumes, page.Volumes...)
			return lastPage
		})
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
		fmt.Println(volumes)
	}
	return volumes
}

func getAddresses(svc *ec2.EC2) []*ec2.Address {
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
	return result.Addresses
}

func captureAddresses(region string, addresses []*ec2.Address) {
	for _, prefix := range []string {"total.", region + "."} {
		key := prefix + "floating_ips"
		stats[key] = stats[key] + int64(len(addresses))
	}
}

func captureInstances(
	region string,
	instances []*ec2.Instance,
	tm map[string] int64,
	states []*string) {

	for _, instance := range instances {
		for _, preprefix := range []string {"total.", region + "."} {
			if _, ok := tm[*instance.InstanceType + ".vcpus"] ; ! ok {
				logErr.Println("Instance type", *instance.InstanceType, "not found.")
			}
			prefix := preprefix + *states[0] + "."
			stats[prefix + "instances"] = stats[prefix + "instances"] + 1
			stats[prefix + "instances." + *instance.InstanceType] = stats[prefix + "instances." + *instance.InstanceType] + 1

			stats[prefix + "vcpus"] = stats[prefix + "vcpus"] + tm[*instance.InstanceType + ".vcpus"]
			stats[prefix + "cores"] = stats[prefix + "cores"] + tm[*instance.InstanceType + ".cores"]
			stats[prefix + "memory_mib"] = stats[prefix + "memory_mib"] + tm[*instance.InstanceType + ".memory"]
		}
	}
}

func getInstances(svc *ec2.EC2, states []*string) []*ec2.Instance {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: states,
			},
		},
	}

	instances := []*ec2.Instance{}
	err := svc.DescribeInstancesPages(input,
		func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			for _, reservation := range(page.Reservations) {
				instances = append(instances, reservation.Instances...)
			}
			return lastPage
		})
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

func getTypes(svc *ec2.EC2) []*ec2.InstanceTypeInfo {

	types := []*ec2.InstanceTypeInfo{}

	output, err := svc.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{})

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
	}


	types = append(types, output.InstanceTypes...)

	for output.NextToken != nil {
		output, err = svc.DescribeInstanceTypes(
			&ec2.DescribeInstanceTypesInput{
				NextToken: output.NextToken,
			})

		types = append(types, output.InstanceTypes...)

	}
	return types
}

func main() {
	if os.Getenv("AWS_PROFILE") == "" {
		os.Setenv("AWS_PROFILE", "gpte")
	}

	if os.Getenv("AWS_REGION") == "" {
		os.Setenv("AWS_REGION", "us-east-1")
	}

	logErr = log.New(os.Stderr, "!!! ", log.LstdFlags)
	logOut = log.New(os.Stdout, "    ", log.LstdFlags)

	sess, _ := session.NewSession(
		&aws.Config{
			Region:     aws.String(os.Getenv("AWS_REGION")),
			MaxRetries: &maxRetries,
		},
	)

	svcGlob := ec2.New(sess)

	types := getTypes(svcGlob)
	regions, _ := svcGlob.DescribeRegions(&ec2.DescribeRegionsInput{})
	tm := buildTypeMap(types)


	for _, region := range regions.Regions {
		sess, _ := session.NewSession(
			&aws.Config{
				Region:     region.RegionName,
				MaxRetries: &maxRetries,
			},
		)
		svc := ec2.New(sess)

		captureVolumes(*region.RegionName, getVolumes(svc))

		captureAddresses(*region.RegionName, getAddresses(svc))

		states := []*string{
			aws.String("running"),
			aws.String("pending"),
		}
		instances := getInstances(svc, states)
		captureInstances(*region.RegionName, instances, tm, states)

		states = []*string{
			aws.String("stopped"),
			aws.String("shutting-down"),
			aws.String("stopping"),
		}
		instances = getInstances(svc, states)
		captureInstances(*region.RegionName, instances, tm, states)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	enc.Encode(&stats)
}
