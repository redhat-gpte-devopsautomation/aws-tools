package main

import (
	"os"
	"encoding/json"
	"strings"
	"fmt"
	"log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var maxRetries = 100
var stats = map[string] int64 {}
var account string

// Logging
var logErr *log.Logger
var logOut *log.Logger

// TODO: cleanup or use this
var (
	completionTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aws_usage_last_completion_timestamp_seconds",
		Help: "The timestamp of the last completion, successful or not.",
	})
	successTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aws_usage_last_success_timestamp_seconds",
		Help: "The timestamp of the last successful completion.",
	})
	duration = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aws_usage_duration_seconds",
		Help: "The duration of the last aws-usage in seconds.",
	})
)

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

func captureVolumes(region string, result []*ec2.Volume, pusher *push.Pusher) {
	sizeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_volumes_size_gib",
			Help: "Total size of volumes",
		})
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_volumes_count",
			Help: "Total number of volumes",
		})
	for _, volume := range result {
		sizeGauge.Add(float64(*volume.Size))
		gauge.Inc()

		for _, prefix := range []string {"total.", region + "."} {
			stats[prefix + "volumes.size_gib"] = stats[prefix + "volumes.size_gib"] + *volume.Size
			stats[prefix + "volumes.count"] = stats[prefix + "volumes.count"] + 1
		}
	}

	pusher.Collector(gauge).Collector(sizeGauge)
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

func captureAddresses(region string, addresses []*ec2.Address, pusher *push.Pusher) {
	if len(addresses) > 0 {
		gauge := prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "aws_usage_floating_ips",
				Help: "Total number of Floating IPs",
			})
		gauge.Set(float64(len(addresses)))
		pusher.Collector(gauge)

		for _, prefix := range []string {"total.", region + "."} {
			key := prefix + "floating_ips"
			stats[key] = stats[key] + int64(len(addresses))
		}
	}
}

func captureInstances(
	region string,
	instances []*ec2.Instance,
	tm map[string] int64,
	states []*string,
	pusher *push.Pusher) {

	instanceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_instances_" + *states[0],
			Help: "Total number of instances " + *states[0],
		})
	coreGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_cores_" + *states[0],
			Help: "Total number of CPU Cores for " + *states[0] + " instances",
		})
	vcpuGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_vcpus_" + *states[0],
			Help: "Total number of VCPUs for " + *states[0] + " instances",
		})
	memoryGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_memory_mib_" + *states[0],
			Help: "Total memory in MiB for " + *states[0] + " instances",
		})
	pusher.Collector(instanceGauge).Collector(coreGauge).
		Collector(vcpuGauge).Collector(memoryGauge)

	// Keep gauges for all types
	countInstancesByType := map[string] float64{}

	for _, instance := range instances {
		instanceGauge.Inc()
		vcpuGauge.Add(float64(tm[*instance.InstanceType + ".vcpus"]))
		coreGauge.Add(float64(tm[*instance.InstanceType + ".cores"]))
		memoryGauge.Add(float64(tm[*instance.InstanceType + ".memory"]))

		countInstancesByType[*instance.InstanceType] = countInstancesByType[*instance.InstanceType] + 1

		if _, ok := tm[*instance.InstanceType + ".vcpus"] ; ! ok {
			logErr.Println("Instance type", *instance.InstanceType, "not found.")
		}
		for _, preprefix := range []string {"total.", region + "."} {
			prefix := preprefix + *states[0] + "."
			stats[prefix + "instances"] = stats[prefix + "instances"] + 1
			stats[prefix + "instances." + *instance.InstanceType] = stats[prefix + "instances." + *instance.InstanceType] + 1

			stats[prefix + "vcpus"] = stats[prefix + "vcpus"] + tm[*instance.InstanceType + ".vcpus"]
			stats[prefix + "cores"] = stats[prefix + "cores"] + tm[*instance.InstanceType + ".cores"]
			stats[prefix + "memory_mib"] = stats[prefix + "memory_mib"] + tm[*instance.InstanceType + ".memory"]
		}
	}

	for instanceType, count := range countInstancesByType {
		instanceTypeGauge := prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "aws_usage_instances_" + strings.ReplaceAll(instanceType, ".", "_") + "_" + *states[0],
				Help: "Total number of instances of type " + instanceType + " " + *states[0],
			})
		instanceTypeGauge.Set(count)
		pusher.Collector(instanceTypeGauge)
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

/* S3 */

func getBuckets(svc *s3.S3) []*s3.Bucket {
	input := &s3.ListBucketsInput{}
	result, err := svc.ListBuckets(input)
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
		return []*s3.Bucket{}
	}
	return result.Buckets
}

func captureBuckets(buckets []*s3.Bucket, pusher *push.Pusher) {
	bucketGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_s3_buckets",
			Help: "Total number of S3 buckets",
		})
	pusher.Collector(bucketGauge)

	bucketGauge.Set(float64(len(buckets)))
	stats["total.s3.buckets"] = int64(len(buckets))
}

func main() {

	logErr = log.New(os.Stderr, "!!! ", log.LstdFlags)
	logOut = log.New(os.Stdout, "    ", log.LstdFlags)

	if os.Getenv("AWS_PROFILE") == "" {
		logErr.Println("AWS_PROFILE env variable must be define")
	}

	account = os.Getenv("AWS_PROFILE")

	if os.Getenv("PROMETHEUS_GATEWAY") == "" {
		logErr.Println("PROMETHEUS_GATEWAY env variable must be define")
	}

	sess, _ := session.NewSession(
		&aws.Config{
			Region:     aws.String("us-east-1"),
			MaxRetries: &maxRetries,
		},
	)

	svcGlob := ec2.New(sess)

	types := getTypes(svcGlob)
	regions, _ := svcGlob.DescribeRegions(&ec2.DescribeRegionsInput{})
	tm := buildTypeMap(types)

	s3svc := s3.New(sess)
	pusherGlobal := push.New(os.Getenv("PROMETHEUS_GATEWAY"), "aws-usage").
		Grouping("account", account)
	captureBuckets(getBuckets(s3svc), pusherGlobal)
	if err := pusherGlobal.Push(); err != nil {
		fmt.Println(err.Error())
	}

	for _, region := range regions.Regions {
		pusher := push.New(os.Getenv("PROMETHEUS_GATEWAY"), "aws-usage").
			Grouping("account", account).Grouping("region", *region.RegionName)

		sess, _ := session.NewSession(
			&aws.Config{
				Region:     region.RegionName,
				MaxRetries: &maxRetries,
			},
		)
		svc := ec2.New(sess)

		captureVolumes(*region.RegionName, getVolumes(svc), pusher)

		captureAddresses(*region.RegionName, getAddresses(svc), pusher)

		states := []*string{
			aws.String("running"),
			aws.String("pending"),
		}
		instances := getInstances(svc, states)
		captureInstances(*region.RegionName, instances, tm, states, pusher)

		states = []*string{
			aws.String("stopped"),
			aws.String("shutting-down"),
			aws.String("stopping"),
		}
		instances = getInstances(svc, states)
		captureInstances(*region.RegionName, instances, tm, states, pusher)
		if err := pusher.Push(); err != nil {
			fmt.Println(err.Error())
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	enc.Encode(&stats)
}
