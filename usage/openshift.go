package main

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

func countClusters(instances []*ec2.Instance, pusher *push.Pusher) int {
	workerCount := map[string]int{}
	for _, instance := range(instances) {
		uuid := ""

		for _, tag := range(instance.Tags) {
			if *tag.Key == "uuid" {
				uuid = *tag.Value
				break
			}
		}

		for _, tag := range(instance.Tags) {
			if *tag.Key == "Name" {
				if strings.Contains(*tag.Value, "master-1") ||
				strings.Contains(*tag.Value,
				"master1") {
					//instanceType = "master"
				}

				if len(*tag.Value) > 4 && (*tag.Value)[:4] == "node" {
					//instanceType = "worker"
					workerCount[uuid] = workerCount[uuid] + 1
				}
				if strings.Contains(*tag.Value, "-worker-") {
					//instanceType = "worker"
					workerCount[uuid] = workerCount[uuid] + 1
				}
			}
		}
	}

	tshirtSize := map[string]float64{}

	for _, v := range workerCount {
		if v <= 2 {
			tshirtSize["training"] = tshirtSize["training"] + 1
			continue
		}

		if v <= 3 {
			tshirtSize["small"] = tshirtSize["small"] + 1
			continue
		}

		if v > 3 {
			tshirtSize["large"] = tshirtSize["large"] + 1
			continue
		}
	}

	clusterGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_openshift_clusters",
			Help: "Total Number of OpenShift clusters",
		})
	trainingClusterGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_openshift_clusters_training",
			Help: "Total Number of OpenShift clusters size Training (2 workers or less)",
		})
	smallClusterGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_openshift_clusters_small",
			Help: "Total Number of OpenShift clusters size Small (3 workers or less)",
		})
	largeClusterGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aws_usage_openshift_clusters_large",
			Help: "Total Number of OpenShift clusters size Large (4 workers or more)",
		})

	pusher.Collector(clusterGauge).Collector(trainingClusterGauge).
		Collector(smallClusterGauge).
		Collector(largeClusterGauge)

	clusterGauge.Set(float64(len(workerCount)))
	trainingClusterGauge.Set(tshirtSize["training"])
	smallClusterGauge.Set(tshirtSize["small"])
	largeClusterGauge.Set(tshirtSize["large"])

	return len(workerCount)
}
