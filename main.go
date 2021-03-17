package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sns"
)

var (
	TopicArn   string  = "arn:aws:sns:us-east-1:12334567:Covid-vaccine" //change default
	Latitude   float64 = 41.6698982
	Longitude  float64 = -91.5983959
	Radius     int     = 200
	AWS_region string  = "us-east-1"
	State      string  = "IA"
)

var fnvHash hash.Hash32 = fnv.New32a()

func main() {
	lambda.Start(HandleRequest) //*IMPORTANT* comment/remove for local testing
	//getVaccine() //*IMPORTANT*  uncomment for local testing
}

//HandleRequest : lambda execution
func HandleRequest(ctx context.Context) (string, error) {
	str, err := getVaccine()
	return str, err
}

func getVaccine() (string, error) {

	STATE := getEnvState()
	rb := JSONBODY{}
	rb.Operationname = "SearchPharmaciesNearPointWithCovidVaccineAvailability"
	rb.Variables.Radius = getEnvRadius()
	rb.Variables.Latitude = Latitude
	rb.Variables.Longitude = Longitude
	rb.Query = "query SearchPharmaciesNearPointWithCovidVaccineAvailability($latitude: Float!, $longitude: Float!, $radius: Int! = 10) {\n  searchPharmaciesNearPoint(latitude: $latitude, longitude: $longitude, radius: $radius) { distance location {   locationId   name   nickname   phoneNumber   businessCode   isCovidVaccineAvailable   covidVaccineEligibilityTerms   address {     line1     line2     city     state     zip     latitude     longitude     __typename   }   __typename } __typename  }}"
	fmt.Printf("RADIUS: %v\n", rb.Variables.Radius)
	fmt.Printf("LATITUDE: %v\n", rb.Variables.Latitude)
	fmt.Printf("LONGTITUDE: %v\n", rb.Variables.Longitude)
	fmt.Printf("STATE: %v\n", getEnvState())
	requestBody, err := json.Marshal(rb)

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", "https://www.hy-vee.com/my-pharmacy/api/graphql", bytes.NewBuffer(requestBody))
	request.Header.Set("Content-type", "application/json")

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	resp, err := client.Do(request)

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//log.Println(string(body))

	bytes := body
	var res RESPBODY
	json.Unmarshal(bytes, &res)

	//fmt.Println(res.Data.Searchpharmaciesnearpoint)
	var available re

	for _, val := range res.Data.Searchpharmaciesnearpoint {
		// fmt.Println("HyVee Vaccine location")
		// fmt.Printf("Location: %s\n", val.Location.Name)
		// fmt.Printf("Address: %s, %s, %s, %s\n", val.Location.Address.Line1, val.Location.Address.City, val.Location.Address.State, val.Location.Address.Zip)
		// fmt.Printf("Is the vaccine available: %t\n", val.Location.Iscovidvaccineavailable)
		// fmt.Printf("Covid Vaccine Eligibility Terms: %s\n", val.Location.Covidvaccineeligibilityterms)
		// fmt.Println()

		if val.Location.Iscovidvaccineavailable && val.Location.Address.State == STATE {
			available = append(available, val)
		}
	}

	if len(available) > 0 {
		for _, val := range available {
			fmt.Println("HyVee Vaccination location")
			fmt.Printf("Location: %s\n", val.Location.Name)
			fmt.Printf("Phone: %s\n", val.Location.Phonenumber)
			fmt.Printf("Address: %s, %s, %s, %s\n", val.Location.Address.Line1, val.Location.Address.City, val.Location.Address.State, val.Location.Address.Zip)
			fmt.Printf("Is the vaccine available: %t\n", val.Location.Iscovidvaccineavailable)
			fmt.Printf("Covid Vaccine Eligibility Terms: %s\n", val.Location.Covidvaccineeligibilityterms)
			fmt.Println()
		}
		message := composeMessage(available)
		fmt.Println(message)
		hash := getHash(message)
		fmt.Println(hash)
		if updateDatabase(hash) {
			return sendMessage(message)
		}
	}

	return "Nothing to do", nil

}

func composeMessage(available re) string {
	//compose message
	var resultStr strings.Builder
	resultStr.WriteString("Vaccination available at:\n")
	for _, val := range available {
		location := fmt.Sprintf("Location: %s\n", val.Location.Name)
		resultStr.WriteString(location)
		hyveeSignUp := fmt.Sprintf("URL: %s\n", "https://www.hy-vee.com/my-pharmacy/covid-vaccine-consent\n")
		resultStr.WriteString(hyveeSignUp)
		phonenumber := fmt.Sprintf("Phone: %s\n", val.Location.Phonenumber)
		resultStr.WriteString(phonenumber)
		address := fmt.Sprintf("Address: %s, %s, %s, %s\n", val.Location.Address.Line1, val.Location.Address.City, val.Location.Address.State, val.Location.Address.Zip)
		resultStr.WriteString(address)
		eligibility := fmt.Sprintf("Covid Vaccine Eligibility Terms: %s\n", val.Location.Covidvaccineeligibilityterms)
		resultStr.WriteString(eligibility)
		resultStr.WriteString("- - -\n")
	}

	return resultStr.String()
}

func sendMessage(message string) (string, error) {

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(AWS_region),
	})

	if err != nil {
		fmt.Println("NewSession error:", err)
		return "Unable to create session", err
	}

	client := sns.New(sess)
	input := &sns.PublishInput{
		Message:  aws.String(message),
		TopicArn: aws.String(getEnvTopic()),
	}

	result, err := client.Publish(input)
	if err != nil {
		fmt.Println("Publish error:", err)
		return "ERROR publishing...", err
	}

	fmt.Println(result)
	output := fmt.Sprintf("%s", result)
	return output, nil
}

func updateDatabase(hash string) bool {

	sess, errSession := session.NewSession(&aws.Config{
		Region: aws.String(AWS_region),
	})

	if errSession != nil {
		fmt.Println("NewSession error:", errSession)
		return false
	}
	// Create DynamoDB client
	svc := dynamodb.New(sess)

	// Update item in table Covid
	tableName := "Covid"
	source := "covid-hyvee-only"
	id := "2020"

	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Source": {
				S: aws.String(source),
			},
			"ID": {
				N: aws.String(id),
			},
		},
	})

	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	if result.Item == nil {
		fmt.Println("Could not find item..continue")
	}
	item := Covid{}

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		panic(fmt.Sprintf("Failed to unmarshal Record, %v", err))
	}

	if result != nil {
		fmt.Println("Found item:")
		fmt.Println("Source:  ", item.Source)
		fmt.Println("fingerprint: ", item.Fingerprint)
		fmt.Println("ID:", item.ID)
		fmt.Println("hash: ", hash)
	}

	if item.Fingerprint == hash {
		fmt.Println("No need to update since nothing changed")
		return false
	} else {
		input := &dynamodb.UpdateItemInput{
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":f": {
					S: aws.String(hash),
				},
			},
			TableName: aws.String(tableName),
			Key: map[string]*dynamodb.AttributeValue{
				"ID": {
					N: aws.String(id),
				},
				"Source": {
					S: aws.String(source),
				},
			},
			ReturnValues:     aws.String("UPDATED_NEW"),
			UpdateExpression: aws.String("set Fingerprint = :f"),
		}

		_, err2 := svc.UpdateItem(input)
		if err2 != nil {
			fmt.Println(err2.Error())
			return false
		}

		fmt.Println("Successfully updated dynamo")
		return true
	}
}

func getHash(s string) string {
	fnvHash.Write([]byte(s))
	defer fnvHash.Reset()

	return fmt.Sprintf("%x", fnvHash.Sum(nil))
}

func getEnvState() string {
	v := os.Getenv("STATE")
	if v == "" {
		return State
	}
	return v
}

func getEnvTopic() string {
	v := os.Getenv("TOPIC_ARN")
	if v == "" {
		return TopicArn
	}
	return v
}

func getEnvRadius() int {
	v := os.Getenv("RADIUS")
	if v == "" {
		return Radius
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return i
}

type re []struct {
	Distance float64 `json:"distance"`
	Location struct {
		Locationid                   string `json:"locationId"`
		Name                         string `json:"name"`
		Nickname                     string `json:"nickname"`
		Phonenumber                  string `json:"phoneNumber"`
		Businesscode                 string `json:"businessCode"`
		Iscovidvaccineavailable      bool   `json:"isCovidVaccineAvailable"`
		Covidvaccineeligibilityterms string `json:"covidVaccineEligibilityTerms"`
		Address                      struct {
			Line1     string      `json:"line1"`
			Line2     interface{} `json:"line2"`
			City      string      `json:"city"`
			State     string      `json:"state"`
			Zip       string      `json:"zip"`
			Latitude  float64     `json:"latitude"`
			Longitude float64     `json:"longitude"`
			Typename  string      `json:"__typename"`
		} `json:"address"`
		Typename string `json:"__typename"`
	} `json:"location"`
	Typename string `json:"__typename"`
}

type RESPBODY struct {
	Data struct {
		Searchpharmaciesnearpoint []struct {
			Distance float64 `json:"distance"`
			Location struct {
				Locationid                   string `json:"locationId"`
				Name                         string `json:"name"`
				Nickname                     string `json:"nickname"`
				Phonenumber                  string `json:"phoneNumber"`
				Businesscode                 string `json:"businessCode"`
				Iscovidvaccineavailable      bool   `json:"isCovidVaccineAvailable"`
				Covidvaccineeligibilityterms string `json:"covidVaccineEligibilityTerms"`
				Address                      struct {
					Line1     string      `json:"line1"`
					Line2     interface{} `json:"line2"`
					City      string      `json:"city"`
					State     string      `json:"state"`
					Zip       string      `json:"zip"`
					Latitude  float64     `json:"latitude"`
					Longitude float64     `json:"longitude"`
					Typename  string      `json:"__typename"`
				} `json:"address"`
				Typename string `json:"__typename"`
			} `json:"location"`
			Typename string `json:"__typename"`
		} `json:"searchPharmaciesNearPoint"`
	} `json:"data"`
	Extensions struct {
		Tracing struct {
			Version   int       `json:"version"`
			Starttime time.Time `json:"startTime"`
			Endtime   time.Time `json:"endTime"`
			Duration  int       `json:"duration"`
			Execution struct {
				Resolvers []interface{} `json:"resolvers"`
			} `json:"execution"`
		} `json:"tracing"`
	} `json:"extensions"`
}

type JSONBODY struct {
	Operationname string `json:"operationName"`
	Variables     struct {
		Radius    int     `json:"radius"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"variables"`
	Query string `json:"query"`
}

type Covid struct {
	ID          int
	Source      string
	Fingerprint string
}
