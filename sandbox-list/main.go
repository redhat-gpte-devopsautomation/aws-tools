package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var csv bool
var all bool

type Account struct {
	Name               string  `json:"name"`
	Available          bool    `json:"available"`
	Guid               string  `json:"guid"`
	Envtype            string  `json:"envtype"`
	AccountId          string  `json:"account_id"`
	Owner              string  `json:"owner"`
	OwnerEmail         string  `json:"owner_email"`
	Zone               string  `json:"zone"`
	HostedZoneId       string  `json:"hosted_zone_id"`
	UpdateTime         float64 `json:"aws:rep:updatetime"`
	Comment            string  `json:"comment"`
	AwsAccessKeyId     string  `json:"aws_access_key_id"`
	AwsSecretAccessKey string  `json:"aws_secret_access_key"`
}

func (a Account) String() string {
	var separator string
	if csv {
		separator = ","
	} else {
		separator = "\t"
	}
	ti, err := strconv.ParseInt(strconv.FormatFloat(a.UpdateTime, 'f', 0, 64), 10, 64)
	if err != nil {
		panic(err)
	}

	updatetime := time.Unix(ti, 0)
	diff := time.Now().Sub(updatetime)
	return strings.Join([]string{
		a.Name,
		strconv.FormatBool(a.Available),
		a.Guid,
		a.Envtype,
		a.AccountId,
		a.Owner,
		a.OwnerEmail,
		a.Zone,
		a.HostedZoneId,
		fmt.Sprintf("%s (%dd)", updatetime.Format(time.RFC3339), int(diff.Hours()/24)),
		a.Comment,
	}, separator)
}

func printHeaders() {
	var separator string
	if csv {
		separator = ","
	} else {
		separator = "\t"
	}

	headers := []string{
		"Name",
		"Available",
		"Guid",
		"Envtype",
		"AccountId",
		"Owner",
		"OwnerEmail",
		"Zone",
		"HostedZoneId",
		"UpdateTime",
		"Comment",
	}
	for _, h := range headers {
		fmt.Printf("%s%s", h, separator)
	}
	fmt.Println()
}

func parseFlags() {
	// Option to show event
	flag.BoolVar(&csv, "csv", false, "Use CSV format to print accounts.")
	flag.BoolVar(&all, "all", false, "Just print all sandboxes.")
	flag.Parse()
}

func buildAccounts(r *dynamodb.ScanOutput) []Account {
	accounts := []Account{}

	for _, sandbox := range r.Items {
		item := Account{}
		err := dynamodbattribute.UnmarshalMap(sandbox, &item)

		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		accounts = append(accounts, item)
	}

	return accounts
}

func used(accounts []Account) []Account {
	r := []Account{}
	for _, i := range accounts {
		if !i.Available {
			r = append(r, i)
		}
	}
	return r
}

func countAvailable(accounts []Account) int {
	total := 0

	for _, sandbox := range accounts {
		if sandbox.Available {
			total = total + 1
		}
	}

	return total
}

func sortUpdateTime(accounts []Account) []Account {
	_accounts := append([]Account{}, accounts...)

	sort.SliceStable(_accounts, func(i, j int) bool {
		return _accounts[i].UpdateTime > _accounts[j].UpdateTime
	})
	return _accounts
}

func countUsed(accounts []Account) int {
	return len(accounts) - countAvailable(accounts)
}

func printMostRecentlyUsed(accounts []Account) {
	m := sortUpdateTime(used(accounts))

	fmt.Println()
	fmt.Println("Most recently used sandboxes:")
	printHeaders()
	for i := 0; i < 10; i++ {
		fmt.Println(m[i])
	}
}

func printBroken(accounts []Account) {
	fmt.Println()
	fmt.Println("Broken sandboxes:")
	printHeaders()
	for _, sandbox := range accounts {
		if sandbox.AwsAccessKeyId == "" {
			fmt.Printf("%v %v\n", sandbox, "Access key missing")
		}
		if sandbox.AwsSecretAccessKey == "" {
			fmt.Printf("%v %v\n", sandbox, "Access secret key missing")
		}
		if sandbox.Zone == "" {
			fmt.Printf("%v %v\n", sandbox, "Zone missing")
		}
		if sandbox.HostedZoneId == "" {
			fmt.Printf("%v %v\n", sandbox, "HostedZoneId missing")
		}
		if !sandbox.Available && sandbox.Owner == "" && sandbox.OwnerEmail == "" {
			fmt.Printf("%v %v\n", sandbox, "Owner missing")
		}
	}
}

func main() {
	parseFlags()

	if os.Getenv("AWS_PROFILE") == "" {
		os.Setenv("AWS_PROFILE", "pool-manager")
	}
	if os.Getenv("AWS_REGION") == "" {
		os.Setenv("AWS_REGION", "us-east-1")
	}
	svc := dynamodb.New(session.New())

	proj := expression.NamesList(
		expression.Name("name"),
		expression.Name("available"),
		expression.Name("guid"),
		expression.Name("envtype"),
		expression.Name("owner"),
		expression.Name("zone"),
		expression.Name("hosted_zone_id"),
		expression.Name("account_id"),
		expression.Name("comment"),
		expression.Name("owner_email"),
		expression.Name("aws:rep:updatetime"),
		expression.Name("aws_access_key_id"),
		expression.Name("aws_secret_access_key"),
	)

	expr, err := expression.NewBuilder().WithProjection(proj).Build()

	if err != nil {
		fmt.Println("Got error building expression:")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	input := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		TableName:                 aws.String("accounts"),
		ProjectionExpression:      expr.Projection(),
	}

	result, err := svc.Scan(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeRequestLimitExceeded:
				fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}

	accounts := buildAccounts(result)
	if all {
		printHeaders()
		for _, sandbox := range sortUpdateTime(accounts) {
			fmt.Println(sandbox)
		}
		os.Exit(0)
	}
	usedAccounts := used(accounts)
	fmt.Println("Total Used:", len(usedAccounts), "/", len(accounts))

	printMostRecentlyUsed(accounts)
	printBroken(accounts)
}
