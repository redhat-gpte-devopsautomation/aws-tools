/*
This program will return all the resources created by a IAM user after a specific time

It uses CloudTrail to find the resources.
Then it returns those who still exist.

See doc at https://docs.aws.amazon.com/sdk-for-go/api/

DONE: list all events done by user and his instances (master0 usually)
DONE: add a recursive option to include all resources created by instances
DONE: make concurrency work (throttling), catch exceptions and retry using (exponentially) delayed retries
DONE: Split into several files for readability/maintenance
DONE: dry-mode: print resources still existing => first step: this will be emailed to us after deletion
DONE: filter out possible false-positive, stupid ex: a user describe our top root route53 domain, we don't want to delete the domain! For now exclude *Describe* actions. Need to comeup with a whitelist of actions.
TODO: make sure concurrency work again with all the *Exists() functions that use different API (ec2, iam, ...)
TODO: all a all-region option to control all possible AWS regions
TODO: delete all resources, including dynamic resources (gp2 storage class, elb...)
TODO: filter out resources if creation time is before time passed as argument
*/

package main

import (
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/connect"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

var userName string
var startTime time.Time
var debug bool
var recursive bool
var showevents bool

// Logs
var logErr *log.Logger
var logOut *log.Logger
var logDebug *log.Logger
var logReport *log.Logger

// clients

var sess client.ConfigProvider
var svcCloudtrail *cloudtrail.CloudTrail

var maxRetries int = 100

func parseFlags() {
	var startTimeString string
	// Option to show event
	flag.BoolVar(&debug, "v", false, "Whether to show DEBUG info")
	flag.BoolVar(&showevents, "showevents", false, "Whether to show Events info")
	flag.BoolVar(&recursive, "r", false, "Perform action recursively, search for resources touched or created by instances which themselves were created by the user")
	flag.StringVar(&userName, "u", "", "The username that created the resources")
	flag.StringVar(&startTimeString, "t", "", "Filter event starting at that time. It's RFC3339 or ISO8601 time, ex: 2019-01-14T09:04:25.392000+00:00")

	flag.Parse()

	if userName == "" || startTimeString == "" {
		flag.PrintDefaults()
		os.Exit(2)
	}
	var err error
	startTime, err = time.Parse(time.RFC3339, startTimeString)
	if err != nil {
		logErr.Println("Error parsing start time")
		os.Exit(1)
	}
}

func v(line ...interface{}) {
	if debug {
		logDebug.Println(line...)
	}
}

func IsInterestingEvent(eventName string) bool {
	switch eventName {
	case
		"RegisterTargets":
		return false
	}

	if strings.Contains(eventName, "Describe") {
		return false
	}
	return true
}

func resourceExists(resource *cloudtrail.Resource) bool {
	switch *resource.ResourceType {
	case "AWS::EC2::Instance":
		return ec2InstanceExists(*resource.ResourceName)
	case "AWS::EC2::Volume":
		return ec2VolumeExists(*resource.ResourceName)
	case "AWS::EC2::NatGateway":
		return ec2NatGatewayExists(*resource.ResourceName)
	case "AWS::EC2::Subnet":
		return ec2SubnetExists(*resource.ResourceName)
	case "AWS::EC2::EIP":
		return ec2EIPExists(*resource.ResourceName)
	case "AWS::EC2::RouteTable":
		return ec2RouteTableExists(*resource.ResourceName)
	case "AWS::EC2::SecurityGroup":
		return ec2SecurityGroupExists(*resource.ResourceName)
	case "AWS::EC2::NetworkInterface":
		return ec2NetworkInterfaceExists(*resource.ResourceName)
	case "AWS::EC2::VPC":
		return ec2VpcExists(*resource.ResourceName)
	case "AWS::EC2::InternetGateway":
		return ec2InternetGatewayExists(*resource.ResourceName)
	case "AWS::EC2::Ami":
		return ec2ImageExists(*resource.ResourceName)
	case "AWS::IAM::InstanceProfile":
		return iamInstanceProfileExists(*resource.ResourceName)
	case "AWS::IAM::Role":
		return iamRoleExists(*resource.ResourceName)
	case "AWS::ElasticLoadBalancing::LoadBalancer":
		return elasticLoadBalancingLoadBalancerExists(*resource.ResourceName)
	case "AWS::ElasticLoadBalancingV2::LoadBalancer":
		return elasticLoadBalancingV2LoadBalancerExists(*resource.ResourceName)
	case "AWS::ElasticLoadBalancingV2::Listener":
		return elasticLoadBalancingV2ListenerExists(*resource.ResourceName)
	case "AWS::ElasticLoadBalancingV2::TargetGroup":
		return elasticLoadBalancingV2TargetGroupExists(*resource.ResourceName)
	case "AWS::S3::Bucket":
		return s3BucketExists(*resource.ResourceName)

		/* TODO:
		   23 AWS::EC2::SubnetRouteTableAssociation
		    3 AWS::IAM::Policy
		*/
	default:
		logErr.Println("Type", *resource.ResourceType, "not supported")
	}

	return false
}

func filterExisting(resources []*cloudtrail.Resource) (result []*cloudtrail.Resource) {
	result = []*cloudtrail.Resource{}

	for _, resource := range resources {
		if resourceExists(resource) {
			result = append(result, resource)
		}
	}

	return result
}

func searchAllResources(svcCloudtrail *cloudtrail.CloudTrail, username string, starttime time.Time) []*cloudtrail.Resource {
	v("searchAllResources(", username, ",", starttime, ")")

	input := &cloudtrail.LookupEventsInput{
		StartTime: &starttime,
		LookupAttributes: []*cloudtrail.LookupAttribute{
			{
				AttributeKey:   aws.String("Username"),
				AttributeValue: &username,
			},
		},
	}
	seen := map[string]bool{}
	resources := []*cloudtrail.Resource{}

	retries := 0
	delay := 1

LookupLoop:
	for retries < maxRetries {
		pageNum := 0
		err := svcCloudtrail.LookupEventsPages(input,
			func(page *cloudtrail.LookupEventsOutput, lastPage bool) bool {
				pageNum++

				for _, event := range page.Events {
					if len(event.Resources) > 0 && IsInterestingEvent(*event.EventName) {
						for _, resource := range event.Resources {
							if resource.ResourceType != nil {
								if !seen[*resource.ResourceName] {
									if showevents {
										v(event)
									}
									resources = append(resources, resource)
									seen[*resource.ResourceName] = true
									v("└──", *resource.ResourceType, *resource.ResourceName)
								}
							}
						}
					}
				}
				randomDelay := time.Duration(rand.Intn(int(delay))) * time.Second
				v(
					"searchAllResources(",
					username,
					",",
					starttime,
					") ",
					"page ",
					pageNum,
					" sleeping ",
					randomDelay,
					"resources ",
					len(resources),
				)
				time.Sleep(randomDelay)
				return pageNum <= 3000 // max 3000 pages ( 3000x50=150000 events )
			})

		if err != nil {
			if err, ok := err.(awserr.Error); ok {
				switch err.Code() {
				case connect.ErrCodeThrottlingException:
					retries++
					randomDelay := time.Duration(rand.Intn(int(delay))) * time.Second
					v("# Throttled because of too many connections... sleeping", randomDelay, "-- Resources found so far: ", len(resources))
					if retries >= maxRetries {
						break LookupLoop
					}
					time.Sleep(randomDelay)
					delay = delay * 2
					continue LookupLoop
				default:
					logErr.Println("Got error calling LookupEvent:")
					logErr.Println(err.Error())
				}
			}

			logErr.Println("Got error calling LookupEvent:")
			logErr.Println(err.Error())
			os.Exit(2)
		} else {
			break LookupLoop
		}
	}
	return resources
}

func filterInstances(resources []*cloudtrail.Resource) []string {
	res := []string{}

	for _, resource := range resources {
		if (*resource.ResourceName)[:2] == "i-" &&
			*resource.ResourceType == "AWS::EC2::Instance" {
			res = append(res, *resource.ResourceName)
		}
	}
	return res
}

func main() {
	parseFlags()

	logErr = log.New(os.Stderr, "!!! ", log.LstdFlags)
	logOut = log.New(os.Stdout, "    ", log.LstdFlags)
	logDebug = log.New(os.Stdout, "(d) ", log.LstdFlags)
	logReport = log.New(os.Stdout, "+++ ", log.LstdFlags)

	var err error
	sess, err = session.NewSession(
		&aws.Config{
			Region:     aws.String(os.Getenv("AWS_REGION")),
			MaxRetries: &maxRetries,
		},
	)

	if err != nil {
		logErr.Println("Got error calling NewSession:")
		logErr.Println(err.Error())
		os.Exit(1)
	}

	svcCloudtrail = cloudtrail.New(sess)

	resources := searchAllResources(svcCloudtrail, userName, startTime)

	if recursive {
		for _, instance := range filterInstances(resources) {
			resources = append(resources, searchAllResources(svcCloudtrail, instance, startTime)...)
		}
	}

	v("Total number of resources to test for existence:", len(resources))
	existingResources := filterExisting(resources)

	if len(existingResources) > 0 {
		logReport.Println("Activity of user", userName, "starting at ", startTime)
		logReport.Println("Number of resources still existing:", len(existingResources))
		logReport.Println()
		for _, resource := range existingResources {
			logReport.Println(*resource.ResourceType, *resource.ResourceName)
		}
	} else {
		logOut.Println("Activity of user", userName, "starting at ", startTime)
		logOut.Println("No resources found.")
	}
}
