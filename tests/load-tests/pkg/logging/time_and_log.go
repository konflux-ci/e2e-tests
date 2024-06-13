package logging

import "fmt"
import "path/filepath"
import "reflect"
import "runtime"
import "sort"
import "strings"
import "time"
import "os"
import "encoding/csv"
import "sync"

var measurementsQueue chan MeasurementEntry // channel to send measurements to
var errorsQueue chan ErrorEntry // chanel to send failures to

var measurementsOutput string // path to CSV where to save measurements
var errorsOutput string // path to CSV where to save measurements

var writerWaitGroup sync.WaitGroup

var batchSize int // when we accumulate this many of records, we dump them to CSV (this is to batch writest to the file, possibly make it faster)

// Represents the data about measurement we want to store to CSV
type MeasurementEntry struct {
	Timestamp  time.Time
	Metric     string
	Duration   time.Duration
	Parameters string
	Error      error
}

// Helper function to convert struct to slice of string which is needed when converting to CSV
func (e *MeasurementEntry) GetSliceOfStrings() []string {
	return []string{e.Timestamp.Format(time.RFC3339Nano), e.Metric, fmt.Sprintf("%f", e.Duration.Seconds()), e.Parameters, fmt.Sprintf("%v", e.Error)}
}

// Represents the data about failure we want to store to CSV
type ErrorEntry struct {
	Timestamp time.Time
	Code      int
	Message   string
}

// Helper function to convert struct to slice of string which is needed when converting to CSV
func (e *ErrorEntry) GetSliceOfStrings() []string {
	return []string{e.Timestamp.Format(time.RFC3339Nano), fmt.Sprintf("%d", e.Code), e.Message}
}


// Initialize channels and start functions that are processing records
func MeasurementsStart(directory string) {
	batchSize = 3

	writerWaitGroup.Add(2)

	measurementsQueue = make(chan MeasurementEntry)
	measurementsOutput = directory + "/load-test-timings.csv"
	go measurementsWriter()

	errorsQueue = make(chan ErrorEntry)
	errorsOutput = directory + "/load-test-errors.csv"
	go errorsWriter()
}

// Close channels and wait to ensure any remaining records are written to CSV
func MeasurementsStop() {
	close(measurementsQueue)
	close(errorsQueue)
	writerWaitGroup.Wait()
}

// Append slice to a CSV file
func writeToCSV(outfile string, batch [][]string) error {
	outfile = filepath.Clean(outfile)
	file, err := os.OpenFile(outfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, d := range batch {
		if err := writer.Write(d); err != nil {
			return err
		}
	}

	return nil
}

// Process measurements comming via channel, batching writes to CSV file
func measurementsWriter() {
	defer writerWaitGroup.Done()

	var counter int
	var batch [][]string

	for {
		event, ok := <-measurementsQueue
		if !ok {
			// Handle channel closure
			break
		}
		batch = append(batch, event.GetSliceOfStrings())
		counter++
		if len(batch) == batchSize {
			err := writeToCSV(measurementsOutput, batch)
			if err != nil {
				Logger.Error("Error writing to CSV file: %v", err)
			}
			batch = make([][]string, batchSize)
		}
	}

	// Write any remaining data to the CSV file
	if len(batch) > 0 {
		err := writeToCSV(measurementsOutput, batch)
		if err != nil {
			Logger.Error("Error writing to CSV file: %v", err)
		}
	}

	Logger.Debug("Finished measurementsWriter, %d measurements processed", counter)
}

// Process failures comming via channel, batching writes to CSV file
// TODO deduplicate this and MeasurementsWriter somehow
func errorsWriter() {
	defer writerWaitGroup.Done()

	var counter int
	var batch [][]string

	for {
		event, ok := <-errorsQueue
		if !ok {
			// Handle channel closure
			break
		}
		batch = append(batch, event.GetSliceOfStrings())
		counter++
		if len(batch) == batchSize {
			err := writeToCSV(errorsOutput, batch)
			if err != nil {
				Logger.Error("Error writing to CSV file: %v", err)
			}
			batch = make([][]string, batchSize)
		}
	}

	// Write any remaining data to the CSV file
	if len(batch) > 0 {
		err := writeToCSV(errorsOutput, batch)
		if err != nil {
			Logger.Error("Error writing to CSV file: %v", err)
		}
	}

	Logger.Debug("Finished errorsWriter, %d errors processed", counter)
}

// Measure duration of a given function run with given parameters and return what function returned
// This only returns first (data) and last (error) returned value. Maybe this
// can be generalized completely, but it is good enough for our needs.
func Measure(fn interface{}, params ...interface{}) (interface{}, error) {
	funcValue := reflect.ValueOf(fn)

	// Construct arguments for the function call
	numParams := len(params)
	args := make([]reflect.Value, numParams)
	for i := 0; i < numParams; i++ {
		args[i] = reflect.ValueOf(params[i])
	}

	// Create map of parameters with key being parameter type (I do not
	// know how to access parameter name) and value parameter value
	// Because of that we are adding index to key to ensure we capture
	// all params even when multiple of params have same type.
	paramsStorable := make(map[string]string)
	for i := 0; i < numParams; i++ {
		x := 1
		key := fmt.Sprintf("%v", reflect.TypeOf(params[i]))
		value := fmt.Sprintf("%+v", reflect.ValueOf(params[i]))
		for {
			keyFull := key + fmt.Sprint(x)
			if _, ok := paramsStorable[keyFull]; !ok {
				paramsStorable[keyFull] = value
				break
			}
			x++
		}
	}

	var errInterValue error
	var resultInterValue interface{}

	// Get function name
	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()

	startTime := time.Now()

	defer func() {
		elapsed := time.Since(startTime)
		LogMeasurement(funcName, paramsStorable, elapsed, fmt.Sprintf("%+v", resultInterValue), errInterValue)
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

// Store given measurement
func LogMeasurement(metric string, params map[string]string, elapsed time.Duration, result string, err error) {
	// Extract parameter keys into a slice so we can sort them
	var paramsKeys []string
	for k := range params {
		paramsKeys = append(paramsKeys, k)
	}

	// Sort parameter keys alphabetically
	sort.Strings(paramsKeys)

	// Construct string showing parameters except for framework that contains token, so we hide it
	var params_string string = ""
	for _, k := range paramsKeys {
		if strings.HasPrefix(k, "*framework.Framework") {
			params_string = params_string + fmt.Sprintf(" %s:redacted", k)
		} else {
			params_string = params_string + fmt.Sprintf(" %s:%s", k, params[k])
		}
	}
	params_string = strings.TrimLeft(params_string, " ")

	Logger.Trace("Measured function: %s, Duration: %s, Params: %s, Result: %s, Error: %v\n", metric, elapsed, params_string, result, err)
	data := MeasurementEntry{
		Timestamp:  time.Now(),
		Metric:     metric,
		Duration:   elapsed,
		Parameters: params_string,
		Error:      err,
	}
	measurementsQueue <- data
}
