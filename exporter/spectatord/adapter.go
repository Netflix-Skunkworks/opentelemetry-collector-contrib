package spectatord

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/gommon/log"
	"go.uber.org/zap"
)

var specTypeMapping = map[metricspb.MetricDescriptor_Type]string{
	metricspb.MetricDescriptor_CUMULATIVE_DOUBLE: "C",
	metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION: "C",
	metricspb.MetricDescriptor_GAUGE_DOUBLE:      "g",
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
	case metricspb.MetricDescriptor_SUMMARY:
		return s.updateSummary(descriptor, series)
	case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION:
		return s.updateCumulativeDistribution(descriptor, series)
	default:
		log.Info("dropping metric of unexpected type %s:%s", descriptor.GetName(), descriptor.Type)
		return nil
	}
}

func (s *Adapter) updateMonotonicCounter(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	return s.updateSingleValue(metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, descriptor, series)
}

func (s *Adapter) updateGauge(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	return s.updateSingleValue(metricspb.MetricDescriptor_GAUGE_DOUBLE, descriptor, series)
}

func (s *Adapter) updateSummary(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	// Example of timeseries data for the summary type
	// {
	//	"start_timestamp": {
	//		"seconds": 1609976233,
	//		"nanos": 410000000
	//	},
	//	"label_values": [{
	//		"value": "prometheus",
	//		"has_value": true
	//	}],
	//	"points": [{
	//		"timestamp": {
	//			"seconds": 1609976248,
	//			"nanos": 410000000
	//		},
	//		"Value": {
	//			"SummaryValue": {
	//				"count": {},
	//				"sum": {},
	//				"snapshot": {
	//					"percentile_values": [{
	//						"percentile": 1,
	//						"value": 0.000011372
	//					}, {
	//						"percentile": 5,
	//						"value": 0.000011372
	//					}, {
	//						"percentile": 50,
	//						"value": 0.000013092
	//					}, {
	//						"percentile": 90,
	//						"value": 0.000045715
	//					}, {
	//						"percentile": 99,
	//						"value": 0.000084335
	//					}]
	//				}
	//			}
	//		}
	//	}]
	//}

	// spectatord doesn't support passing percentiles directly, instead it assumes it computes this itself from a stream
	// of values.  So we must split each of percentile int a gauge metric.  We split below by tagging the metrics with "percentile"

	metricName := descriptor.GetName()

	if len(series.Points) != 1 {
		return fmt.Errorf("skipping update, unexpected number of points in metric: %s:%d", metricName, len(series.Points))
	}

	var result *multierror.Error
	const percentile = "percentile"
	tags := s.getTags(descriptor.GetLabelKeys(), series.GetLabelValues())

	for _, pv := range series.Points[0].GetSummaryValue().Snapshot.PercentileValues {
		tags[percentile] = strconv.FormatFloat(pv.Percentile, 'f', -1, 64)

		result = s.writeSpectatordMsgMultiError(
			metricspb.MetricDescriptor_GAUGE_DOUBLE,
			metricName,
			tags,
			strconv.FormatFloat(pv.Value, 'f', -1, 64),
			result)
	}

	if result != nil {
		return result.ErrorOrNil()
	} else {
		return nil
	}
}

func (s *Adapter) updateCumulativeDistribution(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	if err:= validateCumulativeDistribution(descriptor, series); err != nil {
		log.Warnf("Dropping due to unexpected distribution format, metric: %s, error: %s", descriptor.Name, err)
		return nil
	}

	metricName := descriptor.GetName()
	tags := s.getTags(descriptor.GetLabelKeys(), series.GetLabelValues())
	newDist := series.Points[0].GetDistributionValue()
	bounds := newDist.BucketOptions.GetExplicit().Bounds

	var result *multierror.Error
	const bucketTagKey = "bucket"

	for i, buc := range newDist.Buckets {
		bucketTagValue := "Inf"
		if i < len(bounds) {
			bucketTagValue = strconv.FormatFloat(bounds[i], 'f', -1, 64)
		}

		tags[bucketTagKey] = bucketTagValue
		result = s.writeSpectatordMsgMultiError(
			metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
			metricName,
			tags,
			strconv.FormatInt(buc.Count, 10),
			result)
	}

	delete(tags, bucketTagKey)
	const statisticTagKey = "statistic"

	tags[statisticTagKey] = "totalTime"
	result = s.writeSpectatordMsgMultiError(
		metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
		metricName,
		tags,
		strconv.FormatFloat(newDist.Sum, 'f', -1, 64),
		result)

	if result != nil {
		return result.ErrorOrNil()
	} else {
		return nil
	}
}

func validateCumulativeDistribution(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	metricName := descriptor.GetName()

	if len(series.Points) != 1 {
		return fmt.Errorf("unexpected number of points in metric: %s:%d", metricName, len(series.Points))
	}

	dist := series.Points[0].GetDistributionValue()
	if dist.GetBucketOptions() == nil {
		return fmt.Errorf("no bucket options in metric: %s:%d", metricName, len(series.Points))
	}

	if dist.GetBucketOptions().GetExplicit() == nil {
		return fmt.Errorf("no explicit type in bucket options of metric: %s:%d", metricName, len(series.Points))
	}

	if dist.GetBucketOptions().GetExplicit().GetBounds() == nil {
		return fmt.Errorf("no bounds in bucket options of metric: %s:%d", metricName, len(series.Points))
	}

	if dist.GetBuckets() == nil {
		return fmt.Errorf("no buckets in metric: %s:%d", metricName, len(series.Points))
	}

	return nil
}

func (s *Adapter) updateSingleValue(descType metricspb.MetricDescriptor_Type, descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	metricName := descriptor.GetName()

	if len(series.Points) != 1 {
		return fmt.Errorf("skipping update, unexpected number of points in metric: %s:%d", metricName, len(series.Points))
	}

	tags := s.getTags(descriptor.GetLabelKeys(), series.GetLabelValues())
	s.log.Debug(fmt.Sprintf("tags: %+v", tags))

	newCount := series.Points[0].GetDoubleValue()
	spectatordMsg, err := formatSpectatordMessage(
		specTypeMapping[descType],
		metricName,
		tags,
		strconv.FormatFloat(newCount, 'f', -1, 64))

	if err != nil {
		return fmt.Errorf("failed to format spectatord message: %v", err)
	} else {
		return s.writeSpectatordMsg(spectatordMsg)
	}
}

func (s *Adapter) writeSpectatordMsgMultiError(
	metricType metricspb.MetricDescriptor_Type,
	metricName string,
	tags map[string]string,
	value string,
	result *multierror.Error) *multierror.Error {

	spectatordMsg, err := formatSpectatordMessage(
		specTypeMapping[metricType],
		metricName,
		tags,
		value)

	if err != nil {
		result = multierror.Append(result, fmt.Errorf("failed to format spectatord message: %v", err))
	} else {
		if err = s.writeSpectatordMsg(spectatordMsg); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

func (s *Adapter) writeSpectatordMsg(msg string) error {
	s.log.Debug(fmt.Sprintf("writing to spectatord: %s", msg))
	if _, err := s.conn.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write spectatord message: %v", err)
	}

	return nil
}

func formatSpectatordMessage(metricType, name string, tags map[string]string, value string) (string, error) {
	specTags, err := map2SpectatordTags(tags)
	if err != nil {
		return "", err
	}

	// Since spectatord uses ':' to separate fields. They must be removed from all arguments
	// protocol:metric-type:name#tags:value
	metricType = strings.Replace(metricType, ":", "_", -1)
	name = strings.Replace(name, ":", "_", -1)
	specTags = strings.Replace(specTags, ":", "_", -1)
	// metric-type:name,tags:value@timestamp
	return fmt.Sprintf("%s:%s,%s:%s", metricType, name, specTags, value), nil
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
