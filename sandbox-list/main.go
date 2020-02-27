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
	"text/tabwriter"
	"time"
)

// TODO: use https://golang.org/pkg/text/tabwriter/

var csvFlag bool
var allFlag bool
var toDeleteFlag bool
var noHeadersFlag bool
var padding int = 2

type Account struct {
	Name               string  `json:"name"`
	Available          bool    `json:"available"`
	ToDelete           bool    `json:"to_delete"`
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
	if csvFlag {
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

	var supdatetime string
	if csvFlag {
		supdatetime = updatetime.Format(time.RFC3339)
	} else {
		supdatetime = fmt.Sprintf("%s (%dd)", updatetime.Format("2006-01-02 15:04"), int(diff.Hours()/24))
	}

	var toDeleteString string
	/* Do not write true | false to not break current scripts that filter
           using true|false on the whole line */
	if a.ToDelete {
		toDeleteString = "TODELETE"
	} else {
		toDeleteString = "no"
	}

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
		supdatetime,
		toDeleteString,
		a.Comment,
	}, separator)
}

func printHeaders(w *tabwriter.Writer) {
	if noHeadersFlag {
		return
	}

	var separator string
	if csvFlag {
		separator = ","
	} else {
		separator = "\t"
	}

	headers := []string{
		"Name",
		"Avail",
		"Guid",
		"Envtype",
		"AccountId",
		"Owner",
		"OwnerEmail",
		"Zone",
		"HostedZoneId",
		"UpdateTime",
		"ToDelete?",
		"Comment",
	}
	for _, h := range headers {
		fmt.Fprintf(w, "%s%s", h, separator)
	}
	fmt.Fprintln(w)
}

func parseFlags() {
	// Option to show event
	flag.BoolVar(&csvFlag, "csv", false, "Use CSV format to print accounts.")
	flag.BoolVar(&allFlag, "all", false, "Just print all sandboxes.")
	flag.BoolVar(&toDeleteFlag, "to-delete", false, "Print all marked for deletion.")
	flag.BoolVar(&noHeadersFlag, "no-headers", false, "Don't print headers.")
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', 0)
	m := sortUpdateTime(used(accounts))

	fmt.Println()
	fmt.Println("# Most recently used sandboxes")
	fmt.Println()
	printHeaders(w)
	for i := 0; i < 10; i++ {
		fmt.Fprintln(w, m[i])
	}
	w.Flush()
}

func printOldest(accounts []Account) {
	m := sortUpdateTime(used(accounts))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', 0)

	fmt.Println()
	fmt.Println("# Oldest sandboxes in use")
	fmt.Println()
	printHeaders(w)
	for i := 10; i >= 1; i-- {
		fmt.Fprintln(w, m[len(m)-i])
	}
	w.Flush()
}

func printBroken(accounts []Account) {
	m := []string{}
	for _, sandbox := range accounts {
		if sandbox.AwsAccessKeyId == "" {
			m = append(m, fmt.Sprintf("%v %v\n", sandbox, "Access key missing"))
		}
		if sandbox.AwsSecretAccessKey == "" {
			m = append(m, fmt.Sprintf("%v %v\n", sandbox, "Access secret key missing"))
		}
		if sandbox.Zone == "" {
			m = append(m, fmt.Sprintf("%v %v\n", sandbox, "Zone missing"))
		}
		if sandbox.HostedZoneId == "" {
			m = append(m, fmt.Sprintf("%v %v\n", sandbox, "HostedZoneId missing"))
		}
		if !sandbox.Available && sandbox.Owner == "" && sandbox.OwnerEmail == "" {
			m = append(m, fmt.Sprintf("%v %v\n", sandbox, "Owner missing"))
		}
	}
	if len(m) > 0 {
		fmt.Println()
		fmt.Println("# Broken sandboxes")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', 0)
		printHeaders(w)
		for _, line := range m {
			fmt.Fprint(w, line)
		}
		w.Flush()
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
		expression.Name("to_delete"),
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

	builder := expression.NewBuilder()

	if toDeleteFlag {
		filt := expression.Name("to_delete").Equal(expression.Value(true))
		builder = builder.WithFilter(filt)
	}

	expr, err := builder.WithProjection(proj).Build()

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
		FilterExpression:          expr.Filter(),
	}

	accounts := []Account{}
	errscan := svc.ScanPages(input,
		func(page *dynamodb.ScanOutput, lastPage bool) bool {
			accounts = append(accounts, buildAccounts(page)...)
			return true
		})

	//result, err := svc.Scan(input)
	if errscan != nil {
		if aerr, ok := errscan.(awserr.Error); ok {
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
			fmt.Println(errscan.Error())
		}
		return
	}

	if allFlag || toDeleteFlag {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', 0)
		printHeaders(w)
		for _, sandbox := range sortUpdateTime(accounts) {
			fmt.Fprintln(w, sandbox)
		}
		w.Flush()
		os.Exit(0)
	}

	usedAccounts := used(accounts)
	fmt.Println()
	fmt.Println("Total Used:", len(usedAccounts), "/", len(accounts))

	printMostRecentlyUsed(accounts)
	printOldest(accounts)
	printBroken(accounts)
}
