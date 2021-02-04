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
	"github.com/labstack/gommon/log"
)

var (
	ksClient     *http.Client = nil
	hostname                  = "unknown_hostname"
	stack                     = "unknown_stack"
	instanceId                = "unknown_instance_id"
	ksGatewayURL              = "unknown_gateway_url"
)

const (
	maxMessageSizeBytes = 9000000
	maxEventCount       = 16000
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
	Ec2InstanceId string            `json:"ec2_instance_id"`
	NflxStack     string            `json:"stack"`
	Version       string            `json:"version"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Metadata      map[string]string `json:"metadata"`
	Point         KsPoint           `json:"point"`
}

type KsPoint struct {
	Seconds int64   `json:"seconds"`
	Nanos   int32   `json:"nanos"`
	Value   float64 `json:"value"`
}

func GetEvents(metric *metricspb.Metric) ([]KsEvent, error) {
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

	if metric.Timeseries == nil {
		return nil, fmt.Errorf("no timeseries present in metric: %s", metric.MetricDescriptor.Name)
	}

	events := make([]KsEvent, 0, len(metric.Timeseries))

	for _, series := range metric.Timeseries {
		payload, err := getPayload(metric.MetricDescriptor, series)
		if err != nil {
			log.Errorf("failed to get payload for metric: %s with error: %s", metric.MetricDescriptor.Name, err)
			continue
		}

		event := KsEvent{
			UUID:    uuid.New().String(),
			Payload: *payload,
		}

		events = append(events, event)
	}

	return events, nil
}

func PublishMessages(events []KsEvent) error {
	for {
		head, tail := chunkEvents(events)
		if err := publishMessages(head); err != nil {
			return err
		}

		if len(tail) == 0 {
			return nil
		}

		events = tail
	}
}

func chunkEvents(events []KsEvent) ([]KsEvent, []KsEvent) {
	if len(events) <= maxEventCount {
		return events, []KsEvent{}
	} else {
		return events[:maxEventCount], events[maxEventCount:]
	}
}

func publishMessages(events []KsEvent) error {
	if events == nil || len(events) == 0 {
		return fmt.Errorf("no events provided for construction of message")
	}

	msg := KsMessage{
		AppName:  "otel-contrib-collector.service",
		Hostname: hostname,
		Ack:      false,
		Events:   events,
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for size measurement")
	}

	msgSize := len(b)

	// If the message is small enough return it.
	if msgSize < maxMessageSizeBytes {
		if err := publishMessage(b); err != nil {
			log.Errorf("failed to publish message with error: %s", err)
			return err
		}
		log.Debugf("single message size: %d", msgSize)
		return nil
	}

	log.Infof("splitting message of size: %d", msgSize)

	// If the message is too big split it
	midPoint := len(events) / 2
	if err := publishMessages(events[:midPoint]); err != nil {
		return fmt.Errorf("failed to get left side of split message")
	}

	if err := publishMessages(events[midPoint:]); err != nil {
		return fmt.Errorf("failed to get right side of split message")
	}

	return nil
}

func publishMessage(b []byte) error {
	log.Debugf("publishing keystone message: %s", string(b))

	httpResponse, err := ksClient.Post(ksGatewayURL, "application/json", bytes.NewReader(b))
	if httpResponse == nil {
		return fmt.Errorf("httpResponse from keystone gateway was nil")
	}

	if httpResponse.StatusCode == http.StatusOK && err == nil {
		log.Debugf("published keystone message with size: %d", len(b))
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

func getPayload(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) (*KsPayload, error) {
	metadata, err := getMetadata(descriptor, series)
	if err != nil {
		return nil, err
	}

	if series.Points == nil {
		return nil, fmt.Errorf("no points present in metric: %s", descriptor.Name)
	}

	if len(series.Points) != 1 {
		return nil, fmt.Errorf("unexpected point count in metric: %s, %d != 1", descriptor.Name, len(series.Points))
	}

	point := series.Points[0]

	return &KsPayload{
		Ec2InstanceId: instanceId,
		NflxStack:     stack,
		Version:       "v1",
		Name:          descriptor.Name,
		Type:          descriptor.Type.String(),
		Metadata:      metadata,
		Point: KsPoint{
			Seconds: point.Timestamp.Seconds,
			Nanos:   point.Timestamp.Nanos,
			Value:   point.GetDoubleValue(),
		},
	}, nil
}

func getMetadata(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) (map[string]string, error) {
	metadata := make(map[string]string)

	if descriptor.LabelKeys == nil || series.LabelValues == nil {
		return nil, fmt.Errorf("label keys or values were nil for metric: %s", descriptor.Name)
	}

	if len(descriptor.LabelKeys) != len(series.LabelValues) {
		return nil, fmt.Errorf("length of keys and values does not match for metric: %s", descriptor.Name)
	}

	for i, key := range descriptor.LabelKeys {
		value := series.LabelValues[i]
		if value.HasValue {
			metadata[key.Key] = value.Value
		} else {
			metadata[key.Key] = "MISSING"
		}
	}

	return metadata, nil
}
