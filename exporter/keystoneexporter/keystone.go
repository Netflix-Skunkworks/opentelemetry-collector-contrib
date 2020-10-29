package keystoneexporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/google/uuid"
)

var (
	ksClient     *http.Client = nil
	NilKsMessage              = KsMessage{}
	hostname                  = "unknown_hostname"
	stack                     = "unknown_stack"
	instanceId                = "unknown_instance_id"
	ksGatewayURL              = "unknown_gateway_url"
)

func init() {
	hname, err := os.Hostname()
	if err != nil {
		hostname = "unknown_hostname"
	} else {
		hostname = hname
	}

	if localStack, ok := os.LookupEnv("NETFLIX_STACK"); ok {
		stack = localStack
	} else {
		panic("NETFLIX_STACK not present in environment")
	}

	if localInstanceId, ok := os.LookupEnv("EC2_INSTANCE_ID"); ok {
		instanceId = localInstanceId
	} else {
		panic("EC2_INSTANCE_ID not present in environment")
	}

	ksClient = &http.Client{
		Timeout: 1 * time.Second,
	}

	if ksGatewayURL, err = GetKsGatewayUrl(); err != nil {
		panic(err)
	}
}

type KsEvent struct {
	UUID    string    `json:"uuid"`
	Payload KsPayload `json:"payload"`
}

type KsMessage struct {
	AppName  string    `json:"appName"`
	Hostname string    `json:"hostname"`
	Ack      bool      `json:"ack"`
	Events   []KsEvent `json:"event"`
}

type KsPayload struct {
	Ec2InstanceId string           `json:"ec2_instance_id"`
	NflxStack     string           `json:"stack"`
	Metric        metricspb.Metric `json:"metric"`
}

func GetEvent(metric *metricspb.Metric) (KsEvent, error) {
	// {
	//     "uuid": "123e4567-e89b-a456-426655440000",
	//     "payload": {
	//         "k1": "v1",
	//         "k2": {
	//             "nk1": "v1",
	//             "nk2": "v2"
	//         }
	//     }
	// }

	return KsEvent{
		UUID: uuid.New().String(),
		Payload: KsPayload{
			Ec2InstanceId: instanceId,
			NflxStack:     stack,
			Metric:        *metric,
		},
	}, nil
}

func GetMessage(events []KsEvent) (KsMessage, error) {
	if events == nil || len(events) == 0 {
		return NilKsMessage, fmt.Errorf("no events provided for construction of message")
	}
	return KsMessage{
		AppName:  "otel-contrib-collector.service",
		Hostname: hostname,
		Ack:      false,
		Events:   events,
	}, nil
}

func PublishMessage(msg KsMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	httpResponse, err := ksClient.Post(ksGatewayURL, "application/json", bytes.NewReader(b))
	if httpResponse == nil {
		return fmt.Errorf("httpResponse from keystone gateway was nil")
	}

	if httpResponse.StatusCode == http.StatusOK && err == nil {
		return nil
	}

	if err != nil {
		return err
	}

	bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return err
	}

	return fmt.Errorf(
		"post to ksgateway failed, gatewayURL: %s, status: %d response: %s",
		ksGatewayURL, httpResponse.StatusCode, string(bodyBytes))
}

func GetKsGatewayUrl() (string, error) {
	const regionKey = "EC2_REGION"
	const envKey = "NETFLIX_ENVIRONMENT"
	const streamName = "titus_container_system_metrics"

	region, ok := os.LookupEnv(regionKey)
	if !ok {
		return "", fmt.Errorf("failed to lookup envvar: %s", regionKey)
	}

	env, ok := os.LookupEnv(envKey)
	if !ok {
		return "", fmt.Errorf("failed to lookup envvar: %s", envKey)
	}

	// "https://ksgateway-${REGION}.${ENV}.netflix.net/REST/v1/stream/${STREAM_NAME}"
	return fmt.Sprintf("http://ksgateway-%s.%s.netflix.net/REST/v1/stream/%s", region, env, streamName), nil
}
