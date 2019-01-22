package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"strings"
)

var svcElb *elb.ELB
var svcElbV2 *elbv2.ELBV2

func elasticLoadBalancingLoadBalancerExists(LoadBalancerId string) bool {
	v("exists?", LoadBalancerId)

	// Skip full ids, test only LoadBalancer names
	if strings.Contains(LoadBalancerId, "arn:aws:") {
		return false
	}
	if svcElb == nil {
		svcElb = elb.New(sess)
	}

	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			&LoadBalancerId,
		},
	}
	_, err := svcElb.DescribeLoadBalancers(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "LoadBalancerNotFound":
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

func elasticLoadBalancingV2LoadBalancerExists(LoadBalancerId string) bool {
	v("exists?", LoadBalancerId)

	// Skip full ids, test only LoadBalancer names
	if !strings.Contains(LoadBalancerId, "arn:aws:") {
		return false
	}
	if svcElb == nil {
		svcElbV2 = elbv2.New(sess)
	}

	input := &elbv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []*string{
			aws.String(LoadBalancerId),
		},
	}
	_, err := svcElbV2.DescribeLoadBalancers(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "LoadBalancerNotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
				return false
			}
		} else {
			logErr.Println(err.Error())
			return false
		}
	} else {
		return true
	}
}

func elasticLoadBalancingV2ListenerExists(ListenerId string) bool {
	v("exists?", ListenerId)

	// Skip full ids, test only Listener names
	if !strings.Contains(ListenerId, "arn:aws:") {
		return false
	}
	if svcElb == nil {
		svcElbV2 = elbv2.New(sess)
	}

	input := &elbv2.DescribeListenersInput{
		ListenerArns: []*string{
			aws.String(ListenerId),
		},
	}
	_, err := svcElbV2.DescribeListeners(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "ListenerNotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
				return false
			}
		} else {
			logErr.Println(err.Error())
			return false
		}
	} else {
		return true
	}
}

func elasticLoadBalancingV2TargetGroupExists(TargetGroupId string) bool {
	v("exists?", TargetGroupId)

	// Skip full ids, test only TargetGroup names
	if !strings.Contains(TargetGroupId, "arn:aws:") {
		return false
	}
	if svcElb == nil {
		svcElbV2 = elbv2.New(sess)
	}

	input := &elbv2.DescribeTargetGroupsInput{
		TargetGroupArns: []*string{
			aws.String(TargetGroupId),
		},
	}
	_, err := svcElbV2.DescribeTargetGroups(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "TargetGroupNotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
				return false
			}
		} else {
			logErr.Println(err.Error())
			return false
		}
	} else {
		return true
	}
}
