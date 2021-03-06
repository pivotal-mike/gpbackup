package utils_test

import (
	"io"
	"os"

	"github.com/blang/semver"
	"github.com/greenplum-db/gpbackup/testutils"
	"github.com/greenplum-db/gpbackup/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"
)

var _ = Describe("utils/report tests", func() {
	Describe("ParseErrorMessage", func() {
		It("Parses a CRITICAL error message and returns error code 1", func() {
			errStr := "testProgram:testUser:testHost:000000-[CRITICAL]:-Error Message"
			errMsg, exitCode := utils.ParseErrorMessage(errStr)
			Expect(errMsg).To(Equal("Error Message"))
			Expect(exitCode).To(Equal(1))
		})
		It("Returns error code 0 for an empty error message", func() {
			errMsg, exitCode := utils.ParseErrorMessage("")
			Expect(errMsg).To(Equal(""))
			Expect(exitCode).To(Equal(0))
		})
	})
	Describe("WriteReportFile", func() {
		timestamp := "20170101010101"
		config := utils.BackupConfig{
			BackupVersion:   "0.1.0",
			DatabaseName:    "testdb",
			DatabaseVersion: "5.0.0 build test",
		}
		backupReport := &utils.Report{}
		objectCounts := map[string]int{"tables": 42, "sequences": 1, "types": 1000}
		BeforeEach(func() {
			backupReport = &utils.Report{
				BackupType:   "Unfiltered Full Backup",
				DatabaseSize: "42 MB",
				BackupConfig: config,
			}
			utils.System.OpenFileWrite = func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
				return buffer, nil
			}
		})

		It("writes a report for a successful backup", func() {
			backupReport.WriteReportFile("filename", timestamp, objectCounts, "")
			Expect(buffer).To(gbytes.Say(`Greenplum Database Backup Report

Timestamp Key: 20170101010101
GPDB Version: 5\.0\.0 build test
gpbackup Version: 0\.1\.0

Database Name: testdb
Command Line: .*
Backup Type: Unfiltered Full Backup
Backup Status: Success

Database Size: 42 MB
Count of Database Objects in Backup:
sequences                    1
tables                       42
types                        1000`))
		})
		It("writes a report for a failed backup", func() {
			backupReport.WriteReportFile("filename", timestamp, objectCounts, "Cannot access /tmp/backups: Permission denied")
			Expect(buffer).To(gbytes.Say(`Greenplum Database Backup Report

Timestamp Key: 20170101010101
GPDB Version: 5\.0\.0 build test
gpbackup Version: 0\.1\.0

Database Name: testdb
Command Line: .*
Backup Type: Unfiltered Full Backup
Backup Status: Failure
Backup Error: Cannot access /tmp/backups: Permission denied

Database Size: 42 MB
Count of Database Objects in Backup:
sequences                    1
tables                       42
types                        1000`))
		})
		It("writes a report without database size information", func() {
			backupReport.DatabaseSize = ""
			backupReport.WriteReportFile("filename", timestamp, objectCounts, "")
			Expect(buffer).To(gbytes.Say(`Greenplum Database Backup Report

Timestamp Key: 20170101010101
GPDB Version: 5\.0\.0 build test
gpbackup Version: 0\.1\.0

Database Name: testdb
Command Line: .*
Backup Type: Unfiltered Full Backup
Backup Status: Success

Count of Database Objects in Backup:
sequences                    1
tables                       42
types                        1000`))
		})
	})
	Describe("SetBackupTypeFromFlags", func() {
		var backupReport *utils.Report
		BeforeEach(func() {
			backupReport = &utils.Report{}
		})
		DescribeTable("Backup type classification", func(dataOnly bool, ddlOnly bool, noCompression bool, isSchemaFiltered bool, isTableFiltered bool, withStats bool, expectedType string) {
			backupReport.SetBackupTypeFromFlags(dataOnly, ddlOnly, noCompression, isSchemaFiltered, isTableFiltered, withStats)
			Expect(backupReport.BackupType).To(Equal(expectedType))
		},
			Entry("classifies a default backup",
				false, false, false, false, false, false, "Unfiltered Compressed Full Backup"),
			Entry("classifies a default backup with stats",
				false, false, false, false, false, true, "Unfiltered Compressed Full Backup With Statistics"),
			Entry("classifies a metadata-only backup",
				false, true, false, false, false, false, "Unfiltered Compressed Full Metadata-Only Backup"),
			Entry("classifies a data-only backup",
				true, false, false, false, false, false, "Unfiltered Compressed Full Data-Only Backup"),
			Entry("classifies an uncompressed backup",
				false, false, true, false, false, false, "Unfiltered Uncompressed Full Backup"),
			Entry("classifies an uncompressed metadata-only backup",
				false, true, true, false, false, false, "Unfiltered Uncompressed Full Metadata-Only Backup"),
			Entry("classifies an uncompressed data-only backup",
				true, false, true, false, false, false, "Unfiltered Uncompressed Full Data-Only Backup"),
			Entry("classifies a schema-filtered backup",
				false, false, false, true, false, false, "Schema-Filtered Compressed Full Backup"),
			Entry("classifies a schema-filtered metadata-only backup",
				false, true, false, true, false, false, "Schema-Filtered Compressed Full Metadata-Only Backup"),
			Entry("classifies a schema-filtered data-only backup",
				true, false, false, true, false, false, "Schema-Filtered Compressed Full Data-Only Backup"),
			Entry("classifies an uncompressed schema-filtered backup",
				false, false, true, true, false, false, "Schema-Filtered Uncompressed Full Backup"),
			Entry("classifies an uncompressed schema-filtered metadata-only backup",
				false, true, true, true, false, false, "Schema-Filtered Uncompressed Full Metadata-Only Backup"),
			Entry("classifies an uncompressed schema-filtered data-only backup",
				true, false, true, true, false, false, "Schema-Filtered Uncompressed Full Data-Only Backup"),
			Entry("classifies a table-filtered backup",
				false, false, false, false, true, false, "Table-Filtered Compressed Full Backup"),
			Entry("classifies a table-filtered metadata-only backup",
				false, true, false, false, true, false, "Table-Filtered Compressed Full Metadata-Only Backup"),
			Entry("classifies a table-filtered data-only backup",
				true, false, false, false, true, false, "Table-Filtered Compressed Full Data-Only Backup"),
			Entry("classifies an uncompressed table-filtered backup",
				false, false, true, false, true, false, "Table-Filtered Uncompressed Full Backup"),
			Entry("classifies an uncompressed table-filtered metadata-only backup",
				false, true, true, false, true, false, "Table-Filtered Uncompressed Full Metadata-Only Backup"),
			Entry("classifies an uncompressed table-filtered data-only backup",
				true, false, true, false, true, false, "Table-Filtered Uncompressed Full Data-Only Backup"),
		)
		It("sets properties on the report struct with various flag combinations", func() {
			backupReport.SetBackupTypeFromFlags(true, false, true, false, true, false)
			expectedBackupConfig := utils.BackupConfig{Compressed: false, DataOnly: true, SchemaFiltered: false, TableFiltered: true, MetadataOnly: false, WithStatistics: false}
			testutils.ExpectStructsToMatch(expectedBackupConfig, backupReport.BackupConfig)
			backupReport = &utils.Report{}
			backupReport.SetBackupTypeFromFlags(false, true, false, true, false, true)
			expectedBackupConfig = utils.BackupConfig{Compressed: true, DataOnly: false, SchemaFiltered: true, TableFiltered: false, MetadataOnly: true, WithStatistics: true}
			testutils.ExpectStructsToMatch(expectedBackupConfig, backupReport.BackupConfig)
		})
	})
	Describe("EnsureBackupVersionCompatibility", func() {
		It("Panics if gpbackup version is greater than gprestore version", func() {
			defer testutils.ShouldPanicWithMessage("gprestore 0.1.0 cannot restore a backup taken with gpbackup 0.2.0; please use gprestore 0.2.0 or later.")
			utils.EnsureBackupVersionCompatibility("0.2.0", "0.1.0")
		})
		It("Does not panic if gpbackup version is less than gprestore version", func() {
			utils.EnsureBackupVersionCompatibility("0.1.0", "0.1.3")
		})
		It("Does not panic if gpbackup version equals gprestore version", func() {
			utils.EnsureBackupVersionCompatibility("0.1.0", "0.1.0")
		})
	})
	Describe("EnsureDatabaseVersionCompatibility", func() {
		var restoreVersion utils.GPDBVersion
		BeforeEach(func() {
			semver, _ := semver.Make("5.0.0")
			restoreVersion = utils.GPDBVersion{
				VersionString: "5.0.0-beta.9+dev.129.g4bd4e41 build dev",
				SemVer:        semver,
			}
		})
		It("Panics if backup database major version is greater than restore major version", func() {
			defer testutils.ShouldPanicWithMessage("Cannot restore from GPDB version 6.0.0-beta.9+dev.129.g4bd4e41 build dev to 5.0.0-beta.9+dev.129.g4bd4e41 build dev due to catalog incompatibilities.")
			utils.EnsureDatabaseVersionCompatibility("6.0.0-beta.9+dev.129.g4bd4e41 build dev", restoreVersion)
		})
		It("Does not panic if backup database major version is greater than restore major version", func() {
			utils.EnsureDatabaseVersionCompatibility("4.3.16-beta.9+dev.129.g4bd4e41 build dev", restoreVersion)
		})
		It("Does not panic if backup database major version is equal to restore major version", func() {
			utils.EnsureDatabaseVersionCompatibility("5.0.6-beta.9+dev.129.g4bd4e41 build dev", restoreVersion)
		})
	})
	Describe("Email-related functions", func() {
		reportFileContents := []byte(`Greenplum Database Backup Report

Timestamp Key: 20170101010101`)
		contactsFileContents := []byte(`contact1@example.com
contact2@example.org`)
		contactsList := "contact1@example.com contact2@example.org"

		var (
			testExecutor *testutils.TestExecutor
			testCluster  utils.Cluster
			w            *os.File
			r            *os.File
		)
		BeforeEach(func() {
			r, w, _ = os.Pipe()
			testCluster = testutils.SetDefaultSegmentConfiguration()
			utils.System.OpenFileRead = func(name string, flag int, perm os.FileMode) (utils.ReadCloserAt, error) { return r, nil }
			utils.System.Hostname = func() (string, error) { return "localhost", nil }
			utils.System.Getenv = func(key string) string {
				if key == "HOME" {
					return "home"
				} else {
					return "gphome"
				}
			}
			testExecutor = &testutils.TestExecutor{}
			testCluster.Timestamp = "20170101010101"
			testCluster.Executor = testExecutor
		})
		AfterEach(func() {
			utils.InitializeSystemFunctions()
		})
		Context("ConstructEmailMessage", func() {
			It("adds HTML formatting to the contents of the report file", func() {
				w.Write(reportFileContents)
				w.Close()

				message := utils.ConstructEmailMessage(testCluster, contactsList)
				expectedMessage := `To: contact1@example.com contact2@example.org
Subject: gpbackup 20170101010101 on localhost completed
Content-Type: text/html
Content-Disposition: inline
<html>
<body>
<pre style=\"font: monospace\">
Greenplum Database Backup Report

Timestamp Key: 20170101010101
</pre>
</body>
</html>`
				Expect(message).To(Equal(expectedMessage))
			})
		})
		Context("EmailReport", func() {
			var (
				expectedHomeCmd   = "test -f home/mail_contacts"
				expectedGpHomeCmd = "test -f gphome/bin/mail_contacts"
				expectedMessage   = `echo "To: contact1@example.com contact2@example.org
Subject: gpbackup 20170101010101 on localhost completed
Content-Type: text/html
Content-Disposition: inline
<html>
<body>
<pre style=\"font: monospace\">

</pre>
</body>
</html>" | sendmail -t`
			)
			It("sends no email and raises a warning if no mail_contacts file is found", func() {
				w.Write(contactsFileContents)
				w.Close()

				testExecutor.LocalError = errors.Errorf("exit status 2")

				utils.EmailReport(testCluster)
				Expect(testExecutor.NumExecutions).To(Equal(2))
				Expect(testExecutor.LocalCommands).To(Equal([]string{expectedHomeCmd, expectedGpHomeCmd}))
				Expect(stdout).To(gbytes.Say("Found neither gphome/bin/mail_contacts nor home/mail_contacts"))
			})
			It("sends an email to contacts in $HOME/mail_contacts if only that file is found", func() {
				w.Write(contactsFileContents)
				w.Close()

				testExecutor.ErrorOnExecNum = 2 // Shouldn't hit this case, as it shouldn't be executed a second time
				testExecutor.LocalError = errors.Errorf("exit status 2")

				utils.EmailReport(testCluster)
				Expect(testExecutor.NumExecutions).To(Equal(2))
				Expect(testExecutor.LocalCommands).To(Equal([]string{expectedHomeCmd, expectedMessage}))
				Expect(logfile).To(gbytes.Say("Sending email report to the following addresses: contact1@example.com contact2@example.org"))
			})
			It("sends an email to contacts in $GPHOME/bin/mail_contacts if only that file is found", func() {
				w.Write(contactsFileContents)
				w.Close()

				testExecutor.ErrorOnExecNum = 1
				testExecutor.LocalError = errors.Errorf("exit status 2")

				utils.EmailReport(testCluster)
				Expect(testExecutor.NumExecutions).To(Equal(3))
				Expect(testExecutor.LocalCommands).To(Equal([]string{expectedHomeCmd, expectedGpHomeCmd, expectedMessage}))
				Expect(logfile).To(gbytes.Say("Sending email report to the following addresses: contact1@example.com contact2@example.org"))
			})
			It("sends an email to contacts in $HOME/mail_contacts if a file exists in both $HOME and $GPHOME/bin", func() {
				w.Write(contactsFileContents)
				w.Close()

				utils.EmailReport(testCluster)
				Expect(testExecutor.NumExecutions).To(Equal(2))
				Expect(testExecutor.LocalCommands).To(Equal([]string{expectedHomeCmd, expectedMessage}))
				Expect(logfile).To(gbytes.Say("Sending email report to the following addresses: contact1@example.com contact2@example.org"))
			})
		})
	})
})
