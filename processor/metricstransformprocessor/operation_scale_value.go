package metricstransformprocessor

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"strconv"
)

func (mtp *metricsTransformProcessor) scaleValue(metric *metricspb.Metric, mtpOp internalOperation) {
	op := mtpOp.configOperation
	scaleValue, err := strconv.ParseFloat(op.ScaleArgument, 64)
	if err != nil {
		return
	}

	newTimeseries := make([]*metricspb.TimeSeries, 0)
	for _, timeseries := range metric.Timeseries {
		for _, point := range timeseries.Points {
			if isDouble(metric) {
				newValue := point.GetDoubleValue() * scaleValue
				point.Value = &metricspb.Point_DoubleValue{DoubleValue: newValue}
			}
		}
		newTimeseries = append(newTimeseries, timeseries)
	}

	metric.Timeseries = newTimeseries
}

func isDouble(metric *metricspb.Metric) bool {
	t := metric.GetMetricDescriptor().GetType()
	return t == metricspb.MetricDescriptor_GAUGE_DOUBLE ||
		   t == metricspb.MetricDescriptor_CUMULATIVE_DOUBLE
}
