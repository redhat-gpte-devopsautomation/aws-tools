package main

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var svcEc2 *ec2.EC2

func ec2InstanceExists(instanceId string) bool {
	v("exists?", instanceId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeInstanceStatusInput{
		InstanceIds: []*string{
			&instanceId,
		},
	}
	result, err := svcEc2.DescribeInstanceStatus(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidInstanceID.NotFound":
				return false
			case "InvalidInstanceID.Malformed":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, instance := range result.InstanceStatuses {
		if *instance.InstanceState.Name != "terminated" {
			return true
		}
	}

	return false
}

func ec2VolumeExists(volumeId string) bool {
	v("exists?", volumeId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeVolumeStatusInput{
		VolumeIds: []*string{
			&volumeId,
		},
	}
	result, err := svcEc2.DescribeVolumeStatus(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidVolume.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, volume := range result.VolumeStatuses {
		if volume.VolumeStatus.String() != "" {
			return true
		}
	}

	return false
}

func ec2NatGatewayExists(natgatewayId string) bool {
	v("exists?", natgatewayId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeNatGatewaysInput{
		NatGatewayIds: []*string{
			&natgatewayId,
		},
	}
	result, err := svcEc2.DescribeNatGateways(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NatGatewayNotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, natgateway := range result.NatGateways {
		switch *natgateway.State {
		case "deleted", "deleting":
			return false
		default:
			return true
		}
	}

	return false
}

func ec2SubnetExists(subnetId string) bool {
	v("exists?", subnetId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeSubnetsInput{
		SubnetIds: []*string{
			&subnetId,
		},
	}
	result, err := svcEc2.DescribeSubnets(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidSubnetID.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, subnet := range result.Subnets {
		if *subnet.State != "" {
			return true
		}
	}

	return false
}

func ec2VpcExists(vpcId string) bool {
	v("exists?", vpcId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeVpcsInput{
		VpcIds: []*string{
			&vpcId,
		},
	}
	result, err := svcEc2.DescribeVpcs(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidVpcID.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, vpc := range result.Vpcs {
		if *vpc.State != "" {
			return true
		}
	}

	return false
}

func ec2EIPExists(addressId string) bool {
	v("exists?", addressId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeAddressesInput{
		PublicIps: []*string{
			&addressId,
		},
	}
	result, err := svcEc2.DescribeAddresses(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidParameterValue":
				return false
			case "InvalidAddress.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, address := range result.Addresses {
		if *address.PublicIp != "" {
			return true
		}
	}

	return false
}

func ec2RouteTableExists(routeTableId string) bool {
	v("exists?", routeTableId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{
			&routeTableId,
		},
	}
	result, err := svcEc2.DescribeRouteTables(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidParameterValue":
				return false
			case "InvalidRouteTableID.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for range result.RouteTables {
		return true
	}

	return false
}

func ec2SecurityGroupExists(securityGroupId string) bool {
	v("exists?", securityGroupId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{
			&securityGroupId,
		},
	}
	result, err := svcEc2.DescribeSecurityGroups(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidGroupId.Malformed":
				return false
			case "InvalidGroup.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for range result.SecurityGroups {
		return true
	}

	return false
}

func ec2NetworkInterfaceExists(networkInterfaceId string) bool {
	v("exists?", networkInterfaceId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []*string{
			&networkInterfaceId,
		},
	}
	result, err := svcEc2.DescribeNetworkInterfaces(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidNetworkInterfaceID.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			logErr.Println(err.Error())
		}
		return false
	}

	for range result.NetworkInterfaces {
		return true
	}

	return false
}

func ec2InternetGatewayExists(internetGatewayId string) bool {
	v("exists?", internetGatewayId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeInternetGatewaysInput{
		InternetGatewayIds: []*string{
			&internetGatewayId,
		},
	}
	result, err := svcEc2.DescribeInternetGateways(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidInternetGatewayID.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, internetGateway := range result.InternetGateways {
		if *internetGateway.OwnerId != "" {
			return true
		}
	}

	return false
}

func ec2ImageExists(imageId string) bool {
	v("exists?", imageId)
	if svcEc2 == nil {
		svcEc2 = ec2.New(sess)
	}

	input := &ec2.DescribeImagesInput{
		ImageIds: []*string{
			&imageId,
		},
	}
	result, err := svcEc2.DescribeImages(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidImageID.NotFound":
				return false
			default:
				logErr.Println(aerr.Code())
				logErr.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			logErr.Println(err.Error())
		}
		return false
	}

	for _, image := range result.Images {
		if *image.Public {
			logOut.Println(imageId, "is public, skipping.")
			return false
		}

		switch *image.State {
		case "deleted", "deleting":
			return false
		default:
			return true
		}
	}

	return false
}
