package command_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/log-cache-cli/internal/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Meta", func() {
	var (
		logger      *stubLogger
		httpClient  *stubHTTPClient
		cliConn     *stubCliConnection
		tableWriter *bytes.Buffer
	)

	BeforeEach(func() {
		logger = &stubLogger{}
		httpClient = newStubHTTPClient()
		cliConn = newStubCliConnection()
		cliConn.cliCommandResult = [][]string{{"app-guid"}}
		cliConn.usernameResp = "a-user"
		cliConn.orgName = "organization"
		cliConn.spaceName = "space"
		tableWriter = bytes.NewBuffer(nil)
	})

	It("returns app names with app source guids in alphabetical order", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{
					"source-1": "app-2",
					"source-2": "app-1",
				}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			[]string{"--guid"},
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(1))
		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))

		// Or is required because we don't know the order the keys will come
		// out of the map.
		Expect(cliConn.cliCommandArgs[0][1]).To(Or(
			Equal("/v3/apps?guids=source-1,source-2"),
			Equal("/v3/apps?guids=source-2,source-1"),
		))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  Source  Count   Expired  Cache Duration",
			"source-2   app-1   100000  85008    11m45s",
			"source-1   app-2   100000  85008    11m45s",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("removes headers when not printing to a tty", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{
					"source-1": "app-2",
					"source-2": "app-1",
				}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			[]string{"--guid"},
			httpClient,
			logger,
			tableWriter,
			command.WithMetaNoHeaders(),
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			"source-2  app-1  100000  85008  11m45s",
			"source-1  app-2  100000  85008  11m45s",
			"",
		}))
	})

	It("returns service instance names with service source guids", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2", "source-3"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				// The prefix in the service name is to help test alphabetical
				// ordering.
				capiServiceInstancesResponse(map[string]string{
					"source-2": "aa-service-2",
					"source-3": "ab-service-3",
				}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			[]string{"--guid"},
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		uri, err := url.Parse(cliConn.cliCommandArgs[0][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))

		guidsParam, ok := uri.Query()["guids"]
		Expect(ok).To(BeTrue())
		Expect(len(guidsParam)).To(Equal(1))
		Expect(strings.Split(guidsParam[0], ",")).To(ContainElement("source-1"))

		Expect(cliConn.cliCommandArgs[1]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[1][0]).To(Equal("curl"))

		// Or is required because we don't know the order the keys will come
		// out of the map.
		Expect(cliConn.cliCommandArgs[1][1]).To(Or(
			Equal("/v2/service_instances?guids=source-2,source-3"),
			Equal("/v2/service_instances?guids=source-3,source-2"),
		))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  Source        Count   Expired  Cache Duration",
			"source-2   aa-service-2  100000  85008    11m45s",
			"source-3   ab-service-3  100000  85008    11m45s",
			"source-1   app-1         100000  85008    11m45s",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("does not display the Source ID column by default", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(1))
		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		Expect(cliConn.cliCommandArgs[0][1]).To(Equal("/v3/apps?guids=source-1"))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source  Count   Expired  Cache Duration",
			"app-1   100000  85008    11m45s",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("displays the rate column", func() {
		tailer := func(sourceID string, start, end time.Time) []string {
			switch sourceID {
			case "deadbeef-dead-dead-dead-deaddeafbeef":
				return []string{
					`{"timestamp":"300100000002","sourceId":"deadbeef-dead-dead-dead-deaddeafbeef","counter":{"name":"x","total":"100"},"tags":{"deployment":"other"}}`,
				}
			case "source-1":
				return []string{
					`{"timestamp":"300100000002","sourceId":"source-1","counter":{"name":"x","total":"100"},"tags":{"deployment":"other"}}`,
					`{"timestamp":"300100000003","sourceId":"source-1","counter":{"name":"x","total":"1"},"tags":{"deployment":"cf","__name__":"other","source_id":"other"}}`,
					`{"timestamp":"300100000004","sourceId":"source-1","counter":{"name":"x","total":"100"},"tags":{"deployment":"other"}}`,
					`{"timestamp":"301000000000","sourceId":"source-1","counter":{"name":"other","total":"2"}}`,
					`{"timestamp":"400000101179","sourceId":"source-1","counter":{"name":"x","total":"3"},"tags":{"deployment":"cf"}}`,
				}
			case "source-2":
				return []string{
					`{"timestamp":"300080080103","sourceId":"source-2","counter":{"name":"y","total":"10"}}`,
					`{"timestamp":"301000000000","sourceId":"source-2","gauge":{"metrics":{"other":{"value":7}}}}`,
					`{"timestamp":"400000000000","sourceId":"source-2","gauge":{"metrics":{"y":{"value":12}}}}`,
				}
			default:
				panic("unexpected source-id")
			}
		}

		httpClient.responseBody = []string{
			metaResponseInfo(
				"deadbeef-dead-dead-dead-deaddeafbeef",
				"source-1",
				"source-2",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			tailer,
			[]string{"--noise"},
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source                                Count   Expired  Cache Duration  Rate",
			"app-1                                 100000  85008    11m45s          5",
			"source-2                              100000  85008    11m45s          3",
			"deadbeef-dead-dead-dead-deaddeafbeef  100000  85008    11m45s          1",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("prints source IDs without app names when CAPI doesn't return info", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(2))

		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		uri, err := url.Parse(cliConn.cliCommandArgs[0][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))
		guidsParam, ok := uri.Query()["guids"]
		Expect(ok).To(BeTrue())
		Expect(len(guidsParam)).To(Equal(1))
		Expect(strings.Split(guidsParam[0], ",")).To(ConsistOf("source-1", "source-2"))

		Expect(cliConn.cliCommandArgs[1]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[1][0]).To(Equal("curl"))
		Expect(cliConn.cliCommandArgs[1][1]).To(Equal("/v2/service_instances?guids=source-2"))

		Expect(httpClient.requestCount()).To(Equal(1))
		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source    Count   Expired  Cache Duration",
			"app-1     100000  85008    11m45s",
			"source-2  100000  85008    11m45s",
			"",
		}))
	})

	It("prints meta scoped to apps with guids after names", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"deadbeef-dead-dead-dead-deaddeafbeef",
				"source-2",
				"026fb323-6884-4978-a45f-da188dbf8ecd",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{
					"deadbeef-dead-dead-dead-deaddeafbeef": "app-1",
				}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--scope", "applications"}
		command.Meta(
			context.Background(),
			cliConn,
			nil,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source                                Count   Expired  Cache Duration",
			"app-1                                 100000  85008    11m45s",
			"026fb323-6884-4978-a45f-da188dbf8ecd  100000  85008    11m45s",
			"",
		}))
	})

	It("prints meta scoped to platform", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"source-2",
				"deadbeef-dead-dead-dead-deaddeafbeef",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--scope", "PLATFORM"}
		command.Meta(
			context.Background(),
			cliConn,
			nil,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source    Count   Expired  Cache Duration",
			"source-2  100000  85008    11m45s",
			"",
		}))
	})

	It("prints meta scoped to platform with source GUIDs", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"source-2",
				"deadbeef-dead-dead-dead-deaddeafbeef",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--scope", "PLATFORM", "--guid"}
		command.Meta(
			context.Background(),
			cliConn,
			nil,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  Source    Count   Expired  Cache Duration",
			"source-2   source-2  100000  85008    11m45s",
			"",
		}))
	})

	It("does not request more than 50 guids at a time", func() {
		var guids []string
		for i := 0; i < 51; i++ {
			guids = append(guids, fmt.Sprintf("source-%d", i))
		}

		httpClient.responseBody = []string{
			metaResponseInfo(guids...),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
			{
				capiAppsResponse(map[string]string{}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(4))

		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		uri, err := url.Parse(cliConn.cliCommandArgs[0][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(50))

		Expect(cliConn.cliCommandArgs[1]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[1][0]).To(Equal("curl"))
		uri, err = url.Parse(cliConn.cliCommandArgs[1][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(1))

		Expect(cliConn.cliCommandArgs[2]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[2][0]).To(Equal("curl"))
		uri, err = url.Parse(cliConn.cliCommandArgs[2][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v2/service_instances"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(50))

		Expect(cliConn.cliCommandArgs[3]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[3][0]).To(Equal("curl"))
		uri, err = url.Parse(cliConn.cliCommandArgs[3][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v2/service_instances"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(1))

		// 51 entries, 2 blank lines, "Retrieving..." preamble and table
		// header comes to 55 lines.
		Expect(strings.Split(tableWriter.String(), "\n")).To(HaveLen(55))
	})

	It("uses the LOG_CACHE_ADDR environment variable", func() {
		_ = os.Setenv("LOG_CACHE_ADDR", "https://different-log-cache:8080")
		defer func() { _ = os.Unsetenv("LOG_CACHE_ADDR") }()

		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		u, err := url.Parse(httpClient.requestURLs[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(u.Scheme).To(Equal("https"))
		Expect(u.Host).To(Equal("different-log-cache:8080"))
	})

	It("does not send Authorization header with LOG_CACHE_SKIP_AUTH", func() {
		_ = os.Setenv("LOG_CACHE_SKIP_AUTH", "true")
		defer func() { _ = os.Unsetenv("LOG_CACHE_SKIP_AUTH") }()

		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			context.Background(),
			cliConn,
			nil,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(httpClient.requestHeaders[0]).To(BeEmpty())
	})

	It("fatally logs when it receives too many arguments", func() {
		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				[]string{"extra-arg"},
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid arguments, expected 0, got 1."))
	})

	It("fatally logs when scope is not 'platform', 'applications' or 'all'", func() {
		args := []string{"--scope", "invalid"}
		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				args,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Scope must be 'platform', 'applications' or 'all'."))
	})

	It("fatally logs when getting ApiEndpoint fails", func() {
		cliConn.apiEndpointErr = errors.New("some-error")

		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Could not determine Log Cache endpoint: some-error`))
	})

	It("fatally logs when CAPI request fails", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{nil}
		cliConn.cliCommandErr = []error{errors.New("some-error")}

		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Failed to read application information: some-error`))
	})

	It("fatally logs when username cannot be found", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
		}
		cliConn.cliCommandErr = nil

		cliConn.usernameErr = errors.New("some-error")

		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Could not get username: some-error`))
	})

	It("fatally logs when CAPI response is not proper JSON", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{{"invalid"}}
		cliConn.cliCommandErr = nil

		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Failed to read application information: `))
	})

	It("fatally logs when Meta fails", func() {
		httpClient.responseErr = errors.New("some-error")

		Expect(func() {
			command.Meta(
				context.Background(),
				cliConn,
				nil,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Failed to read Meta information: some-error`))
	})
})

func metaResponseInfo(sourceIDs ...string) string {
	var metaInfos []string
	for _, sourceID := range sourceIDs {
		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "100000",
		  "expired": "85008",
		  "oldestTimestamp": "1519256157847077020",
		  "newestTimestamp": "1519256863126668345"
		}`, sourceID))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}

func capiAppsResponse(apps map[string]string) string {
	var resources []string
	for appID, appName := range apps {
		resources = append(resources, fmt.Sprintf(`{"guid": "%s", "name": "%s"}`, appID, appName))
	}
	return fmt.Sprintf(`{ "resources": [%s] }`, strings.Join(resources, ","))
}

func capiServiceInstancesResponse(services map[string]string) string {
	var resources []string
	for serviceID, serviceName := range services {
		resource := fmt.Sprintf(`{"metadata": {"guid": "%s"}, "entity": {"name": "%s"}}`, serviceID, serviceName)
		resources = append(resources, resource)
	}
	return fmt.Sprintf(`{ "resources": [%s] }`, strings.Join(resources, ","))
}
