package main

import (
	"os"
	"encoding/json"
	"strings"
	"fmt"
	"flag"
	"log"
	"github.com/aws/aws-sdk-go/aws"
	"io/ioutil"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"time"
)

var maxRetries = 100
var stats = map[string] int64 {}
var account string

// Logging
var logErr *log.Logger
var logOut *log.Logger
var logProfile *log.Logger

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
	desc string,
	pusher *push.Pusher) {

	instanceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_instances_" + desc,
			Help: "Total number of instances " + desc,
		})
	coreGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_cores_" + desc,
			Help: "Total number of CPU Cores for " + desc + " instances",
		})
	vcpuGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_vcpus_" + desc,
			Help: "Total number of VCPUs for " + desc + " instances",
		})
	memoryGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_memory_mib_" + desc,
			Help: "Total memory in MiB for " + desc + " instances",
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
			prefix := preprefix + desc + "."
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
				Name: "aws_usage_instances_" + strings.ReplaceAll(instanceType, ".", "_") + "_" + desc,
				Help: "Total number of instances of type " + instanceType + " " + desc,
			})
		instanceTypeGauge.Set(count)
		pusher.Collector(instanceTypeGauge)
	}
}

func getInstances(svc *ec2.EC2, filters []*ec2.Filter) []*ec2.Instance {
	input := &ec2.DescribeInstancesInput{}
	if len(filters) > 0 {
		input = &ec2.DescribeInstancesInput{
			Filters: filters,
		}
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

func filterInstancesByState(instances []*ec2.Instance, states []*string) []*ec2.Instance {
	result := []*ec2.Instance{}
INSTANCES:
	for _, instance := range instances {
		for _, state := range states {
			if *instance.State.Name == *state {
				result = append(result, instance)
				continue INSTANCES
			}
		}
	}
	return result
}

func main() {

	logErr = log.New(os.Stderr, "!!! ", log.LstdFlags)
	logOut = log.New(os.Stdout, "    ", log.LstdFlags)

	f, err := os.OpenFile("aws-usage-profile.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	var s3Flag bool
	var addressFlag bool
	var volumeFlag bool
	var profileFlag bool
	var resetFlag bool
	flag.BoolVar(&s3Flag, "s3", true, "enable s3")
	flag.BoolVar(&addressFlag, "addresses", true, "look for floating IPs")
	flag.BoolVar(&volumeFlag, "volumes", true, "look for volumes")
	flag.BoolVar(&profileFlag, "profile", false, "Enable profiling")
	flag.BoolVar(&resetFlag, "reset", false, "Reset stats to zero for the account")

	flag.Parse()

	if profileFlag {
		logProfile = log.New(f, "", log.LstdFlags)
	} else {
		logProfile = log.New(ioutil.Discard, "(d) ", log.LstdFlags)
	}
	if os.Getenv("AWS_PROFILE") == "" {
		logErr.Println("AWS_PROFILE env variable must be define")
	}

	account = os.Getenv("AWS_PROFILE")
	sandbox := ""

	// If account is a sandbox, then save the information under the same "account"
	if len(account) > 7 && account[0:7] == "sandbox" {
		sandbox = account
		account = "sandboxes"
	}

	if os.Getenv("PROMETHEUS_GATEWAY") == "" {
		logErr.Println("PROMETHEUS_GATEWAY env variable must be define")
	}

	sess, _ := session.NewSession(
		&aws.Config{
			Region:     aws.String("us-east-1"),
			MaxRetries: &maxRetries,
		},
	)


	if s3Flag {
		pusherGlobal := push.New(os.Getenv("PROMETHEUS_GATEWAY"), "aws-usage").
			Grouping("account", account)

		if sandbox != "" {
			pusherGlobal.Grouping("sandbox", sandbox)
		}
		if resetFlag {
			pusherGlobal.Delete()
		} else {
			s3svc := s3.New(sess)
			start := time.Now()
			captureBuckets(getBuckets(s3svc), pusherGlobal)
			logProfile.Println("getBuckets+captureBuckets:", time.Since(start))
			if err := pusherGlobal.Push(); err != nil {
				fmt.Println(err.Error())
			}
		}
	}

	if resetFlag {
		// Delete all
		pusher := push.New(os.Getenv("PROMETHEUS_GATEWAY"), "aws-usage").
			Grouping("account", account)

		if sandbox != "" {
			pusher.Grouping("sandbox", sandbox)
		}
		pusher.Delete()
		return
	}
	svcGlob := ec2.New(sess)
	types := getTypes(svcGlob)
	start := time.Now()
	tm := buildTypeMap(types)
	logProfile.Println("buildTypeMap:", time.Since(start))

	regions, _ := svcGlob.DescribeRegions(&ec2.DescribeRegionsInput{})
	for _, region := range regions.Regions {
		pusher := push.New(os.Getenv("PROMETHEUS_GATEWAY"), "aws-usage").
			Grouping("account", account)

		if sandbox != "" {
			pusher.Grouping("sandbox", sandbox)
		}

		pusher.Grouping("region", *region.RegionName)

		sess, _ := session.NewSession(
			&aws.Config{
				Region:     region.RegionName,
				MaxRetries: &maxRetries,
			},
		)
		svc := ec2.New(sess)

		if volumeFlag {
			start = time.Now()
			captureVolumes(*region.RegionName, getVolumes(svc), pusher)
			logProfile.Println("getVolumes+captureVolumes", time.Since(start))
		}

		if addressFlag {
			start = time.Now()
			captureAddresses(*region.RegionName, getAddresses(svc), pusher)
			logProfile.Println("getAddresses+captureAddresses", time.Since(start))
		}


		// All Instances
		states := []*string{
			aws.String("stopped"),
			aws.String("shutting-down"),
			aws.String("stopping"),
			aws.String("running"),
			aws.String("pending"),
		}
		filters := []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: states,
			},
		}
		start = time.Now()
		instances := getInstances(svc, filters)
		logProfile.Println("getInstances all", time.Since(start))

		// Instances stopped

		states = []*string{
			aws.String("stopped"),
			aws.String("shutting-down"),
			aws.String("stopping"),
		}
		start = time.Now()
		instancesStopped := filterInstancesByState(instances, states)
		logProfile.Println("getInstances stopped", time.Since(start))
		start = time.Now()
		captureInstances(*region.RegionName, instancesStopped, tm, "stopped", pusher)
		logProfile.Println(account, *region.RegionName, "captureInstances stopped", time.Since(start))

		// Instances running
		//
		states = []*string{
			aws.String("running"),
			aws.String("pending"),
		}
		start = time.Now()
		instancesRunning := filterInstancesByState(instances, states)
		logProfile.Println("getInstances running", time.Since(start))

		start = time.Now()
		captureInstances(*region.RegionName, instancesRunning, tm, "running", pusher)
		logProfile.Println("captureInstances running", time.Since(start))

		// Running ocp4-cluster
		filters = []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: states,
			},
			{
				Name: aws.String("tag:env_type"),
				Values: []*string{
					aws.String("ocp4-cluster"),
					aws.String("ocp4-workshop"),
					aws.String("ocp-workshop"),
				},
			},
		}
		start = time.Now()

		instancesOCP := instancesRunning
		// If there are running instances
		if len(instancesRunning) != 0 {
			// get those which are from an OpenShift Cluster
			instancesOCP = getInstances(svc, filters)
		}
		captureInstances(*region.RegionName, instancesOCP, tm, "running_ocp_cluster", pusher)
		logProfile.Println("getInstance+captureInstances ocp", time.Since(start))
		if err := pusher.Push(); err != nil {
			fmt.Println(err.Error())
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	enc.Encode(&stats)
}
