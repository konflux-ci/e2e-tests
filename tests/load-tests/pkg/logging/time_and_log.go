package logging

import "fmt"
import "reflect"
import "runtime"
import "time"
import "os"
import "encoding/csv"
import "sync"


// Channel to send measurements to
var measurementsQueue chan MeasurementEntry
var errorsQueue chan ErrorEntry
var measurementsOutput string
var errorsOutput string
var writerWaitGroup sync.WaitGroup
var batchSize int

// Data struct represents the data about measurement to be stored
type MeasurementEntry struct {
	Timestamp  time.Time
	Metric     string
	Duration   time.Duration
	Parameters string
	Error      error
}
func (e *MeasurementEntry) GetSliceOfStrings() []string {
	return []string{e.Timestamp.Format(time.RFC3339Nano), e.Metric, fmt.Sprintf("%f", e.Duration.Seconds()), e.Parameters, fmt.Sprintf("%v", e.Error)}
}

// Data struct represents the data about failure to be stored
type ErrorEntry struct {
	Timestamp  time.Time
	Code       int
	Message    string
}
func (e *ErrorEntry) GetSliceOfStrings() []string {
	return []string{e.Timestamp.Format(time.RFC3339Nano), fmt.Sprintf("%d", e.Code), e.Message}
}

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

func MeasurementsStop() {
	close(measurementsQueue)
	close(errorsQueue)
	writerWaitGroup.Wait()
}

func writeToCSV(outfile string, batch [][]string) error {
	file, err := os.OpenFile(outfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

func LogMeasurement(metric string, params map[string]string, elapsed time.Duration, result string, err error) {
	Logger.Trace("Measured function: %s, Duration: %s, Result: %s, Error: %v\n", metric, elapsed, result, err)
	data := MeasurementEntry{
		Timestamp:  time.Now(),
		Metric:     metric,
		Duration:   elapsed,
		Parameters: fmt.Sprintf("%+v", params),
		Error:      err,
	}
	measurementsQueue <- data
}
