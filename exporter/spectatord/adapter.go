package spectatord

import (
	"fmt"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"net"
	"go.uber.org/zap"
	"strconv"
	"strings"
)

const (
	defaultSpecatordProtocol    = "1"
)

var specTypeMapping = map[metricspb.MetricDescriptor_Type]string{
	metricspb.MetricDescriptor_CUMULATIVE_DOUBLE: "C",
	metricspb.MetricDescriptor_GAUGE_DOUBLE: "g",
}

type Adapter struct {
	log  *zap.Logger
	conn net.Conn
}

func NewAdapter(log *zap.Logger, conn net.Conn) (*Adapter, error) {
	return &Adapter{
		log:  log,
		conn: conn,
	}, nil
}

func (s *Adapter) Start() error {
	return nil
}

func (s *Adapter) Shutdown() error {
	return s.conn.Close()
}

func (s *Adapter) UpdateTimeSeries(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	switch descriptor.GetType() {
	case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
		return s.updateMonotonicCounter(descriptor, series)
	case metricspb.MetricDescriptor_GAUGE_DOUBLE:
		return s.updateGauge(descriptor, series)
	default:
		return fmt.Errorf("dropping metric of unexpected type %s:%s", descriptor.GetName(), descriptor.Type)
	}
}

func (s *Adapter) updateMonotonicCounter(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	metricName := descriptor.GetName()

	if len(series.Points) != 1 {
		return fmt.Errorf("skipping update, unexpected number of points in counter metric: %d", len(series.Points))
	}

	tags := s.getTags(descriptor.GetLabelKeys(), series.GetLabelValues())
	s.log.Debug(fmt.Sprintf("tags: %+v", tags))

	newCount := series.Points[0].GetDoubleValue()
	spectatordMsg, err := formatSpectatordMessage(
		defaultSpecatordProtocol,
		specTypeMapping[metricspb.MetricDescriptor_CUMULATIVE_DOUBLE],
		metricName,
		tags,
		strconv.FormatFloat(newCount, 'f', 2, 64))

	if err != nil {
		return fmt.Errorf("failed to format spectatord message: %v", err)
	} else {
		s.log.Debug(fmt.Sprintf("writing to spectatord: %s", spectatordMsg))
		if _, err := s.conn.Write([]byte(spectatordMsg)); err != nil {
			return fmt.Errorf("failed to write spectatord message to socket: %v", err)
		}
	}

	return nil
}

func (s *Adapter) updateGauge(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	metricName := descriptor.GetName()

	if len(series.Points) != 1 {
		return fmt.Errorf("skipping update, unexpected number of points in gauge metric: %d", len(series.Points))
	}

	tags := s.getTags(descriptor.GetLabelKeys(), series.GetLabelValues())
	s.log.Debug(fmt.Sprintf("tags: %+v", tags))

	newCount := series.Points[0].GetDoubleValue()
	spectatordMsg, err := formatSpectatordMessage(
		defaultSpecatordProtocol,
		specTypeMapping[metricspb.MetricDescriptor_GAUGE_DOUBLE],
		metricName,
		tags,
		strconv.FormatFloat(newCount, 'f', 2, 64))

	if err != nil {
		return fmt.Errorf("failed to format spectatord message: %v", err)
	} else {
		s.log.Debug(fmt.Sprintf("writing to spectatord: %s", spectatordMsg))
		if _, err := s.conn.Write([]byte(spectatordMsg)); err != nil {
			return fmt.Errorf("failed to write spectatord message to socket: %v", err)
		}
	}

	return nil
}

func formatSpectatordMessage(protocol, metricType, name string, tags map[string]string, value string) (string, error) {
	specTags, err := map2SpectatordTags(tags)
	if err != nil {
		return "", err
	}

	// Since spectatord uses ':' to separate fields. They must be removed from all arguments
	// protocol:metric-type:name#tags:value
	protocol = strings.Replace(protocol, ":", "_", -1)
	metricType = strings.Replace(metricType, ":", "_", -1)
	name = strings.Replace(name, ":", "_", -1)
	specTags = strings.Replace(specTags, ":", "_", -1)
	return fmt.Sprintf("%s:%s:%s:#%s:%s", protocol, metricType, name, specTags, value), nil
}

func map2SpectatordTags(tags map[string]string) (string, error) {
	strs := make([]string, 0, len(tags))
	for k, v := range tags {
		strs = append(strs, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(strs, ","), nil
}

func (s *Adapter) getTags(labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) map[string]string {
	tags := map[string]string{}
	for i, key := range labelKeys {
		if labelValues[i].HasValue {
			tags[key.GetKey()] = labelValues[i].Value
		}
	}

	return tags
}
