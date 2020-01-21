package main

import (
	"os"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var sess client.ConfigProvider
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

func capture(
	region string,
	result *ec2.DescribeInstancesOutput,
	types *ec2.DescribeInstanceTypesOutput,
	states []*string) {

	tm := buildTypeMap(types)

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			for _, preprefix := range []string {"total.", region + "."} {
				prefix := preprefix + *states[0] + "."
				stats[prefix + "instance." + *instance.InstanceType] = stats[prefix + "instance." + *instance.InstanceType] + 1

				stats[prefix + "vcpus"] = stats[prefix + "vcpus"] + tm[*instance.InstanceType + ".vcpus"]
				stats[prefix + "cores"] = stats[prefix + "cores"] + tm[*instance.InstanceType + ".cores"]
				stats[prefix + "memory_mib"] = stats[prefix + "memory_mib"] + tm[*instance.InstanceType + ".memory"]
			}
		}
	}
}

func getInstances(region *string, states []*string) *ec2.DescribeInstancesOutput {
	sess, _ := session.NewSession(
		&aws.Config{
			Region:     region,
			MaxRetries: &maxRetries,
		},
	)

	svc := ec2.New(sess)

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: states,
			},
		},
	}

	instances, err := svc.DescribeInstances(input)
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

	sess, _ = session.NewSession(
		&aws.Config{
			Region:     aws.String(os.Getenv("AWS_REGION")),
			MaxRetries: &maxRetries,
		},
	)

	svcGlob := ec2.New(sess)

	types, _ := svcGlob.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{})
	regions, _ := svcGlob.DescribeRegions(&ec2.DescribeRegionsInput{})

	for _, region := range regions.Regions {
		states := []*string{
			aws.String("running"),
			aws.String("pending"),
		}
		instances := getInstances(region.RegionName, states)
		capture(*region.RegionName, instances, types, states)

		states = []*string{
			aws.String("stopped"),
			aws.String("shutting-down"),
			aws.String("stopping"),
		}
		instances = getInstances(region.RegionName, states)
		capture(*region.RegionName, instances, types, states)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.Encode(&stats)
}
