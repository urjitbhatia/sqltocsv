package sqltocsv_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/joho/sqltocsv"
)

// Fatalf interface for easy testing
type tester interface {
	Fatalf(string, ...interface{})
	Errorf(string, ...interface{})
}

func init() {
	os.Setenv("TZ", "UTC")
}

func TestWriteFile(t *testing.T) {
	checkQueryAgainstResult(t, func(rows *sql.Rows) string {
		testCsvFileName := "/tmp/test.csv"
		err := sqltocsv.WriteFile(testCsvFileName, rows)
		if err != nil {
			t.Fatalf("error in WriteCsvToFile: %v", err)
		}

		bytes, err := ioutil.ReadFile(testCsvFileName)
		if err != nil {
			t.Fatalf("error reading %v: %v", testCsvFileName, err)
		}

		return string(bytes[:])
	})
}

func TestWrite(t *testing.T) {
	checkQueryAgainstResult(t, func(rows *sql.Rows) string {
		buffer := &bytes.Buffer{}

		err := sqltocsv.Write(buffer, rows)
		if err != nil {
			t.Fatalf("error in WriteCsvToWriter: %v", err)
		}

		return buffer.String()
	})
}

func TestWriteString(t *testing.T) {
	checkQueryAgainstResult(t, func(rows *sql.Rows) string {

		csv, err := sqltocsv.WriteString(rows)
		if err != nil {
			t.Fatalf("error in WriteCsvToWriter: %v", err)
		}

		return csv
	})
}

func TestWriteHeaders(t *testing.T) {
	converter := getConverter(t)

	converter.WriteHeaders = false

	expected := "Alice,1,1973-11-29 21:33:09 +0000 UTC\n"
	actual := converter.String()

	assertCsvMatch(t, expected, actual)
}

func TestSetHeaders(t *testing.T) {
	converter := getConverter(t)

	converter.Headers = []string{"Name", "Age", "Birthday"}

	expected := "Name,Age,Birthday\nAlice,1,1973-11-29 21:33:09 +0000 UTC\n"
	actual := converter.String()

	assertCsvMatch(t, expected, actual)
}

func TestSetRowPreProcessorModifyingRows(t *testing.T) {
	converter := getConverter(t)

	converter.SetRowPreProcessor(func(rows []string, columnNames []string) (bool, []string) {
		return true, []string{rows[0], "X", "X"}
	})

	expected := "name,age,bdate\nAlice,X,X\n"
	actual := converter.String()

	assertCsvMatch(t, expected, actual)
}

func TestSetRowPreProcessorOmittingRows(t *testing.T) {
	converter := getConverter(t)

	converter.SetRowPreProcessor(func(rows []string, columnNames []string) (bool, []string) {
		return false, []string{}
	})

	expected := "name,age,bdate\n"
	actual := converter.String()

	assertCsvMatch(t, expected, actual)
}

func TestSetTimeFormat(t *testing.T) {
	converter := getConverter(t)

	// Kitchen: 3:04PM
	converter.TimeFormat = time.Kitchen

	expected := "name,age,bdate\nAlice,1,9:33PM\n"
	actual := converter.String()

	assertCsvMatch(t, expected, actual)
}

func TestConvertingNilValueShouldReturnEmptyString(t *testing.T) {
	converter := sqltocsv.New(getTestRowsByQuery(t, "SELECT|people|name,nickname,age|"))

	expected := "name,nickname,age\nAlice,,1\n"
	actual := converter.String()

	assertCsvMatch(t, expected, actual)
}

func checkQueryAgainstResult(t tester, innerTestFunc func(*sql.Rows) string) {
	rows := getTestRows(t)

	expected := "name,age,bdate\nAlice,1,1973-11-29 21:33:09 +0000 UTC\n"

	actual := innerTestFunc(rows)

	assertCsvMatch(t, expected, actual)
}

func getTestRows(t tester) *sql.Rows {
	return getTestRowsByQuery(t, "SELECT|people|name,age,bdate|")
}

func getTestRowsByQuery(t tester, query string) *sql.Rows {
	db := setupDatabase(t)

	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("error querying: %v", err)
	}

	return rows
}

func getConverter(t *testing.T) *sqltocsv.Converter {
	return sqltocsv.New(getTestRows(t))
}

func setupDatabase(t tester) *sql.DB {
	db, err := sql.Open("test", "foo")
	if err != nil {
		t.Fatalf("Error opening testdb %v", err)
	}
	exec(t, db, "WIPE")
	exec(t, db, "CREATE|people|name=string,age=int32,bdate=datetime,nickname=nullstring")
	exec(t, db, "INSERT|people|name=Alice,age=?,bdate=?,nickname=?", 1, time.Unix(123456789, 0), nil)
	return db
}

func exec(t tester, db *sql.DB, query string, args ...interface{}) {
	_, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("Exec of %q: %v", query, err)
	}
}

func assertCsvMatch(t tester, expected string, actual string) {
	if actual != expected {
		t.Errorf("Expected CSV:\n\n%v\n Got CSV:\n\n%v\n", expected, actual)
	}
}

func BenchmarkWrite(b *testing.B) {
	db := setupDatabase(b)
	// Add 10000 rows
	expected := "name,age,bdate\nAlice,1,1973-11-29 21:33:09 +0000 UTC\n"
	for i := 0; i < 10000; i++ {
		exec(b, db, "INSERT|people|name=Alice,age=?,bdate=?,nickname=?", i, time.Unix(123456789, 0), nil)
		expected += fmt.Sprintf("Alice,%d,1973-11-29 21:33:09 +0000 UTC\n", i)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		rows, err := db.Query("SELECT|people|name,age,bdate|")
		if err != nil {
			b.Error(err)
		}
		buffer := &bytes.Buffer{}
		err = sqltocsv.Write(buffer, rows)
		if err != nil {
			b.Fatalf("error in WriteCsvToWriter: %v", err)
		}
		assertCsvMatch(b, expected, buffer.String())
	}
}
