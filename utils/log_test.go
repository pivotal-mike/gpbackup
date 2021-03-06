package utils_test

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"reflect"
	"time"

	"github.com/greenplum-db/gpbackup/testutils"
	"github.com/greenplum-db/gpbackup/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("utils/log tests", func() {
	var (
		testLogger   *utils.Logger
		sampleLogger *utils.Logger
		fakeInfo     os.FileInfo
	)

	BeforeEach(func() {
		err := utils.System.MkdirAll("/tmp/log_dir", 0755)
		Expect(err).ToNot(HaveOccurred())
		fakeInfo, err = os.Stat("/tmp/log_dir")
		Expect(err).ToNot(HaveOccurred())

		utils.System.OpenFileWrite = func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) { return buffer, nil }
		utils.System.CurrentUser = func() (*user.User, error) { return &user.User{Username: "testUser", HomeDir: "testDir"}, nil }
		utils.System.Getpid = func() int { return 0 }
		utils.System.Hostname = func() (string, error) { return "testHost", nil }
		utils.System.IsNotExist = func(err error) bool { return false }
		utils.System.Now = func() time.Time { return time.Date(2017, time.January, 1, 1, 1, 1, 1, time.Local) }
		utils.System.Stat = func(name string) (os.FileInfo, error) { return fakeInfo, nil }
	})
	AfterEach(func() {
		utils.System = utils.InitializeSystemFunctions()
		utils.SetLogger(logger)
	})

	Describe("InitializeLogging", func() {
		BeforeEach(func() {
			sampleLogger = utils.NewLogger(os.Stdout, os.Stderr, buffer, "testDir/gpAdminLogs/testProgram_20170101.log",
				utils.LOGINFO, "testProgram:testUser:testHost:000000-[%s]:-")
		})
		Context("Logger initialized with default log directory and Info log level", func() {
			It("creates a new logger writing to gpAdminLogs and sets utils.logger to this new logger", func() {
				newLogger := utils.InitializeLogging("testProgram", "")
				testLogger = utils.GetLogger()
				if testLogger == nil || !(newLogger == testLogger) {
					Fail("Created logger was not assigned to utils.logger")
				}
				if !reflect.DeepEqual(newLogger, sampleLogger) {
					Fail(fmt.Sprintf("Created logger does not match sample logger:\n%v\n%v", newLogger, sampleLogger))
				}
			})
		})
		Context("Logger initialized with a specified log directory and Info log level", func() {
			It("creates a new logger writing to the specified log directory and sets utils.logger to this new logger", func() {
				sampleLogger = utils.NewLogger(os.Stdout, os.Stderr, buffer, "/tmp/log_dir/testProgram_20170101.log",
					utils.LOGINFO, "testProgram:testUser:testHost:000000-[%s]:-")
				newLogger := utils.InitializeLogging("testProgram", "/tmp/log_dir")
				testLogger = utils.GetLogger()
				if testLogger == nil || !(newLogger == testLogger) {
					Fail("Created logger was not assigned to utils.logger")
				}
				if !reflect.DeepEqual(newLogger, sampleLogger) {
					Fail(fmt.Sprintf("Created logger does not match sample logger:\n%v\n%v", newLogger, sampleLogger))
				}
			})
		})
		Context("Directory or log file does not exist or is not writable", func() {
			It("creates a log directory if given a nonexistent log directory", func() {
				calledWith := ""
				utils.System.IsNotExist = func(err error) bool { return true }
				utils.System.Stat = func(name string) (os.FileInfo, error) {
					calledWith = name
					return fakeInfo, errors.New("file does not exist")
				}
				utils.InitializeLogging("testProgram", "/tmp/log_dir")
				Expect(calledWith).To(Equal("/tmp/log_dir"))
			})
			It("creates a log file if given a nonexistent log file", func() {
				calledWith := ""
				utils.System.OpenFileWrite = func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
					calledWith = name
					return buffer, nil
				}
				utils.System.IsNotExist = func(err error) bool { return true }
				utils.System.Stat = func(name string) (os.FileInfo, error) { return fakeInfo, errors.New("file does not exist") }
				utils.InitializeLogging("testProgram", "/tmp/log_dir")
				Expect(calledWith).To(Equal("/tmp/log_dir/testProgram_20170101.log"))
			})
			It("panics if given a non-writable log directory", func() {
				utils.System.Stat = func(name string) (os.FileInfo, error) { return fakeInfo, errors.New("permission denied") }
				defer testutils.ShouldPanicWithMessage("permission denied")
				utils.InitializeLogging("testProgram", "/tmp/log_dir")
			})
			It("panics if given a non-writable log file", func() {
				utils.System.OpenFileWrite = func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
					return nil, errors.New("permission denied")
				}
				defer testutils.ShouldPanicWithMessage("permission denied")
				utils.InitializeLogging("testProgram", "/tmp/log_dir")
			})
		})
	})
	Describe("GetLogPrefix", func() {
		It("returns a prefix for the current time", func() {
			expectedMessage := "20170101:01:01:01 testProgram:testUser:testHost:000000-[INFO]:-"
			prefix := logger.GetLogPrefix("INFO")
			Expect(expectedMessage).To(Equal(prefix))
		})
	})
	Describe("Output function tests", func() {
		patternExpected := "20170101:01:01:01 testProgram:testUser:testHost:000000-[%s]:-"
		infoExpected := fmt.Sprintf(patternExpected, "INFO")
		warnExpected := fmt.Sprintf(patternExpected, "WARNING")
		verboseExpected := fmt.Sprintf(patternExpected, "DEBUG")
		debugExpected := fmt.Sprintf(patternExpected, "DEBUG")
		errorExpected := fmt.Sprintf(patternExpected, "ERROR")
		fatalExpected := fmt.Sprintf(patternExpected, "CRITICAL")

		Describe("Verbosity set to Error", func() {
			BeforeEach(func() {
				logger.SetVerbosity(utils.LOGERROR)
			})

			Context("Info", func() {
				It("prints to the log file", func() {
					expectedMessage := "error info"
					logger.Info(expectedMessage)
					testutils.NotExpectRegexp(stdout, infoExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, infoExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, infoExpected+expectedMessage)
				})
			})
			Context("Warn", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "error warn"
					logger.Warn(expectedMessage)
					testutils.ExpectRegexp(stdout, warnExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, warnExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, warnExpected+expectedMessage)
				})
			})
			Context("Verbose", func() {
				It("prints to the log file", func() {
					expectedMessage := "error verbose"
					logger.Verbose(expectedMessage)
					testutils.NotExpectRegexp(stdout, verboseExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, verboseExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, verboseExpected+expectedMessage)
				})
			})
			Context("Debug", func() {
				It("prints to the log file", func() {
					expectedMessage := "error debug"
					logger.Debug(expectedMessage)
					testutils.NotExpectRegexp(stdout, debugExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, debugExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, debugExpected+expectedMessage)
				})
			})
			Context("Error", func() {
				It("prints to stderr and the log file", func() {
					expectedMessage := "error error"
					logger.Error(expectedMessage)
					testutils.NotExpectRegexp(stdout, errorExpected+expectedMessage)
					testutils.ExpectRegexp(stderr, errorExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, errorExpected+expectedMessage)
				})
			})
			Context("Fatal", func() {
				It("prints to the log file, then panics", func() {
					expectedMessage := "error fatal"
					defer func() {
						testutils.NotExpectRegexp(stdout, fatalExpected+expectedMessage)
						testutils.NotExpectRegexp(stderr, fatalExpected+expectedMessage)
						testutils.ExpectRegexp(logfile, fatalExpected+expectedMessage)
					}()
					defer testutils.ShouldPanicWithMessage(expectedMessage)
					logger.Fatal(errors.New(expectedMessage), "")
				})
			})
		})
		Describe("Verbosity set to Info", func() {
			BeforeEach(func() {
				logger.SetVerbosity(utils.LOGINFO)
			})

			Context("Info", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "info info"
					logger.Info(expectedMessage)
					testutils.ExpectRegexp(stdout, infoExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, infoExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, infoExpected+expectedMessage)
				})
			})
			Context("Warn", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "info warn"
					logger.Warn(expectedMessage)
					testutils.ExpectRegexp(stdout, warnExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, warnExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, warnExpected+expectedMessage)
				})
			})
			Context("Verbose", func() {
				It("prints to the log file", func() {
					expectedMessage := "info verbose"
					logger.Verbose(expectedMessage)
					testutils.NotExpectRegexp(stdout, verboseExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, verboseExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, verboseExpected+expectedMessage)
				})
			})
			Context("Debug", func() {
				It("prints to the log file", func() {
					expectedMessage := "info debug"
					logger.Debug(expectedMessage)
					testutils.NotExpectRegexp(stdout, debugExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, debugExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, debugExpected+expectedMessage)
				})
			})
			Context("Error", func() {
				It("prints to stderr and the log file", func() {
					expectedMessage := "info error"
					logger.Error(expectedMessage)
					testutils.NotExpectRegexp(stdout, errorExpected+expectedMessage)
					testutils.ExpectRegexp(stderr, errorExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, errorExpected+expectedMessage)
				})
			})
			Context("Fatal", func() {
				It("prints to the log file, then panics", func() {
					expectedMessage := "info fatal"
					defer func() {
						testutils.NotExpectRegexp(stdout, fatalExpected+expectedMessage)
						testutils.NotExpectRegexp(stderr, fatalExpected+expectedMessage)
						testutils.ExpectRegexp(logfile, fatalExpected+expectedMessage)
					}()
					defer testutils.ShouldPanicWithMessage(expectedMessage)
					logger.Fatal(errors.New(expectedMessage), "")
				})
			})
		})
		Describe("Verbosity set to Verbose", func() {
			BeforeEach(func() {
				logger.SetVerbosity(utils.LOGVERBOSE)
			})

			Context("Info", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "verbose info"
					logger.Info(expectedMessage)
					testutils.ExpectRegexp(stdout, infoExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, infoExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, infoExpected+expectedMessage)
				})
			})
			Context("Warn", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "verbose warn"
					logger.Warn(expectedMessage)
					testutils.ExpectRegexp(stdout, warnExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, warnExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, warnExpected+expectedMessage)
				})
			})
			Context("Verbose", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "verbose verbose"
					logger.Verbose(expectedMessage)
					testutils.ExpectRegexp(stdout, verboseExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, verboseExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, verboseExpected+expectedMessage)
				})
			})
			Context("Debug", func() {
				It("prints to the log file", func() {
					expectedMessage := "verbose debug"
					logger.Debug(expectedMessage)
					testutils.NotExpectRegexp(stdout, debugExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, debugExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, debugExpected+expectedMessage)
				})
			})
			Context("Error", func() {
				It("prints to stderr and the log file", func() {
					expectedMessage := "verbose error"
					logger.Error(expectedMessage)
					testutils.NotExpectRegexp(stdout, errorExpected+expectedMessage)
					testutils.ExpectRegexp(stderr, errorExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, errorExpected+expectedMessage)
				})
			})
			Context("Fatal", func() {
				It("prints to the log file, then panics", func() {
					expectedMessage := "verbose fatal"
					defer func() {
						testutils.NotExpectRegexp(stdout, fatalExpected+expectedMessage)
						testutils.NotExpectRegexp(stderr, fatalExpected+expectedMessage)
						testutils.ExpectRegexp(logfile, fatalExpected+expectedMessage)
					}()
					defer testutils.ShouldPanicWithMessage(expectedMessage)
					logger.Fatal(errors.New(expectedMessage), "")
				})
			})
		})
		Describe("Verbosity set to Debug", func() {
			BeforeEach(func() {
				logger.SetVerbosity(utils.LOGDEBUG)
			})

			Context("Info", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "debug info"
					logger.Info(expectedMessage)
					testutils.ExpectRegexp(stdout, infoExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, infoExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, infoExpected+expectedMessage)
				})
			})
			Context("Warn", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "debug warn"
					logger.Warn(expectedMessage)
					testutils.ExpectRegexp(stdout, warnExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, warnExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, warnExpected+expectedMessage)
				})
			})
			Context("Verbose", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "debug verbose"
					logger.Verbose(expectedMessage)
					testutils.ExpectRegexp(stdout, verboseExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, verboseExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, verboseExpected+expectedMessage)
				})
			})
			Context("Debug", func() {
				It("prints to stdout and the log file", func() {
					expectedMessage := "debug debug"
					logger.Debug(expectedMessage)
					testutils.ExpectRegexp(stdout, debugExpected+expectedMessage)
					testutils.NotExpectRegexp(stderr, debugExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, debugExpected+expectedMessage)
				})
			})
			Context("Error", func() {
				It("prints to stderr and the log file", func() {
					expectedMessage := "debug error"
					logger.Error(expectedMessage)
					testutils.NotExpectRegexp(stdout, errorExpected+expectedMessage)
					testutils.ExpectRegexp(stderr, errorExpected+expectedMessage)
					testutils.ExpectRegexp(logfile, errorExpected+expectedMessage)
				})
			})
			Context("Fatal", func() {
				It("prints to the log file, then panics", func() {
					expectedMessage := "debug fatal"
					defer func() {
						testutils.NotExpectRegexp(stdout, fatalExpected+expectedMessage)
						testutils.NotExpectRegexp(stderr, fatalExpected+expectedMessage)
						testutils.ExpectRegexp(logfile, fatalExpected+expectedMessage)
					}()
					defer testutils.ShouldPanicWithMessage(expectedMessage)
					logger.Fatal(errors.New(expectedMessage), "")
				})
			})
		})
	})
	Describe("NewProgressBar", func() {
		It("will print when passed a value that the progress bar should show", func() {
			progressBar := utils.NewProgressBar(10, "test progress bar", true)
			Expect(progressBar.NotPrint).To(Equal(false))
		})
		It("will not print when passed a value that the progress bar should not show", func() {
			progressBar := utils.NewProgressBar(10, "test progress bar", false)
			Expect(progressBar.NotPrint).To(Equal(true))
		})
	})
})
