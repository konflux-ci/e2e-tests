package timeandlog

import "fmt"
import "reflect"
import "runtime"
import "time"
import "os"
import "encoding/csv"
import "sync"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"


// Channel to send measurements to
var MeasurementsQueue chan MeasurementEntry
var MeasurementsOutput string
var MeasurementWaitGroup sync.WaitGroup

// Data struct represents the data to be stored
type MeasurementEntry struct {
	Timestamp  time.Time
	Metric     string
	Duration   time.Duration
	Parameters string
	Error      error
}

func MeasurementsStart(directory string) {
	MeasurementWaitGroup.Add(1)
	MeasurementsQueue = make(chan MeasurementEntry)
	MeasurementsOutput = directory + "/load-test-timings.csv"
	go MeasurementsWriter()
}

func MeasurementsStop() {
	close(MeasurementsQueue)
	MeasurementWaitGroup.Wait()
}

func writeToCSV(batch []MeasurementEntry) error {
	file, err := os.OpenFile(MeasurementsOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, d := range batch {
		record := []string{d.Timestamp.Format(time.RFC3339Nano), d.Metric, fmt.Sprintf("%f", d.Duration.Seconds()), d.Parameters, fmt.Sprintf("%v", d.Error)}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func MeasurementsWriter() {
	defer MeasurementWaitGroup.Done()

	var batch []MeasurementEntry

	for {
		event, ok := <-MeasurementsQueue
		if !ok {
			// Handle channel closure
			break
		}
		batch = append(batch, event)
		if len(batch) == 3 {
			err := writeToCSV(batch)
			if err != nil {
				logging.Logger.Error("Error writing to CSV file: %v", err)
			}
			batch = []MeasurementEntry{}
		}
	}

	// Write any remaining data to the CSV file
	if len(batch) > 0 {
		err := writeToCSV(batch)
		if err != nil {
			logging.Logger.Error("Error writing to CSV file: %v", err)
		}
	}
}



func Measure(fn interface{}, params ...interface{}) (interface{}, error) {
	funcValue := reflect.ValueOf(fn)

	// Construct arguments for the function call
	numParams := len(params)
	args := make([]reflect.Value, numParams)
	for i := 0; i < numParams; i++ {
		args[i] = reflect.ValueOf(params[i])
	}

	var paramsStorable map[string]string
	paramsStorable = make(map[string]string)
	for i := 0; i < numParams; i++ {
		key := fmt.Sprintf("%v", reflect.TypeOf(params[i]))
		value := fmt.Sprintf("%+v", reflect.ValueOf(params[i]))
		paramsStorable[key] = value
	}

	var errInterValue error
	var resultInterValue interface{}

	// Get function name
	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()

	startTime := time.Now()

	defer func() {
		elapsed := time.Since(startTime)
		LogMeasurement(funcName, fmt.Sprintf("%+v", paramsStorable), elapsed, fmt.Sprintf("%+v", resultInterValue), errInterValue)
	}()

	// Call the function with provided arguments
	results := funcValue.Call(args)

	// Extract and return results
	if len(results) == 0 {
		return nil, nil
	}
	errInter := results[len(results)-1]
	if errInter.Interface() == nil {
		resultInterValue = results[0].Interface()
		return resultInterValue, nil
	}
	errInterValue = errInter.Interface().(error)
	return nil, errInterValue
}

func LogMeasurement(metric, params string, elapsed time.Duration, result string, err error) {
	logging.Logger.Trace("Measured function: %s, Params: %s, Duration: %s, Result: %s, Error: %v\n", metric, params, elapsed, result, err)
	data := MeasurementEntry{
		Timestamp:  time.Now(),
		Metric:     metric,
		Duration:   elapsed,
		Parameters: fmt.Sprintf("%+v", params),
		Error:      err,
	}
	MeasurementsQueue <- data
}
