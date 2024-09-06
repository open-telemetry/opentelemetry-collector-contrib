// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// Test_MarshalXML tests that the ParseXML function can parse XML strings into a pcommon.Map,
// and that the MarshalXML function can serialize the pcommon.Map back into an XML string.
// Due to limitations with the Go XML package, the XML strings must be formatted in a specific way,
// most notably that self-closing tags are not supported.
func Test_MarshalXML(t *testing.T) {
	tests := []struct {
		name        string
		flatten     bool
		xml         string
		createError string
		parseError  string
	}{

		{
			name: "Simple example",
			xml:  "<Log><User><ID>00001</ID><Name>Joe</Name><Email>joe.smith@example.com</Email></User><Text>User did a thing</Text></Log>",
		},
		{
			name: "Formatted example",
			xml: `
<Log>
	<User>
	<ID>00001</ID>
	<Name>Joe</Name>
	<Email>joe.smith@example.com</Email>
	</User>
	<Text>User did a thing</Text>
</Log>`,
		},
		{
			name: "Multiple tags with the same name",
			xml:  `<Log>This record has a collision<User id="0001"></User><User id="0002"></User></Log>`,
		},
		// TODO figure out if this case needs testing and/or the parsing is correct
		// {
		// 	name: "Multiple lines of content",
		// 	xml: `<Log>
		// 			This record has multiple lines of
		// 			<User id="0001"></User>
		// 			text content
		// 		  </Log>`,
		// },
		{
			name: "Attribute only element",
			xml:  `<HostInfo hostname="example.com" zone="east-1" cloudprovider="aws"></HostInfo>`,
		},
		{
			name: "Windows Event Log XML",
			xml: `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
					<System>
						<Provider Name="Microsoft-Windows-Security-Auditing" Guid="{54849625-5478-4994-a5ba-3e3b0328c30d}"></Provider>
						<EventID>4625</EventID>
						<Version>0</Version>
						<Level>0</Level>
						<Task>12544</Task>
						<Opcode>0</Opcode>
						<Keywords>0x8010000000000000</Keywords>
						<TimeCreated SystemTime="2024-08-15T20:03:15.9454501Z"></TimeCreated>
						<EventRecordID>57220</EventRecordID>
						<Correlation ActivityID="{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}"></Correlation>
						<Execution ProcessID="656" ThreadID="4652"></Execution>
						<Channel>Security</Channel>
						<Computer>samuel-windows</Computer>
						<Security></Security>
					</System>
					<EventData>
						<Data Name="SubjectUserSid">S-1-0-0</Data>
						<Data Name="SubjectUserName">-</Data>
						<Data Name="SubjectDomainName">-</Data>
						<Data Name="SubjectLogonId">0x0</Data>
						<Data Name="TargetUserSid">S-1-0-0</Data>
						<Data Name="TargetUserName">Administrator</Data>
						<Data Name="TargetDomainName">domain</Data>
						<Data Name="Status">0xc000006d</Data>
						<Data Name="FailureReason">%%2313</Data>
						<Data Name="SubStatus">0xc000006a</Data>
						<Data Name="LogonType">3</Data>
						<Data Name="LogonProcessName">NtLmSsp</Data>
						<Data Name="AuthenticationPackageName">NTLM</Data>
						<Data Name="WorkstationName">-</Data>
						<Data Name="TransmittedServices">-</Data>
						<Data Name="LmPackageName">-</Data>
						<Data Name="KeyLength">0</Data>
						<Data Name="ProcessId">0x0</Data>
						<Data Name="ProcessName">-</Data>
						<Data Name="IpAddress">103.151.140.135</Data>
						<Data Name="IpPort">0</Data>
					</EventData>
				</Event>`,
		},
		{
			name:    "Windows Event Log XML, flattened",
			flatten: true,
			xml: `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
					<System>
						<Provider Name="Microsoft-Windows-Security-Auditing" Guid="{54849625-5478-4994-a5ba-3e3b0328c30d}"></Provider>
						<EventID>4625</EventID>
						<Version>0</Version>
						<Level>0</Level>
						<Task>12544</Task>
						<Opcode>0</Opcode>
						<Keywords>0x8010000000000000</Keywords>
						<TimeCreated SystemTime="2024-08-15T20:03:15.9454501Z"></TimeCreated>
						<EventRecordID>57220</EventRecordID>
						<Correlation ActivityID="{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}"></Correlation>
						<Execution ProcessID="656" ThreadID="4652"></Execution>
						<Channel>Security</Channel>
						<Computer>samuel-windows</Computer>
						<Security></Security>
					</System>
					<EventData>
						<Data Name="SubjectUserSid">S-1-0-0</Data>
						<Data Name="SubjectUserName">-</Data>
						<Data Name="SubjectDomainName">-</Data>
						<Data Name="SubjectLogonId">0x0</Data>
						<Data Name="TargetUserSid">S-1-0-0</Data>
						<Data Name="TargetUserName">Administrator</Data>
						<Data Name="TargetDomainName">domain</Data>
						<Data Name="Status">0xc000006d</Data>
						<Data Name="FailureReason">%%2313</Data>
						<Data Name="SubStatus">0xc000006a</Data>
						<Data Name="LogonType">3</Data>
						<Data Name="LogonProcessName">NtLmSsp</Data>
						<Data Name="AuthenticationPackageName">NTLM</Data>
						<Data Name="WorkstationName">-</Data>
						<Data Name="TransmittedServices">-</Data>
						<Data Name="LmPackageName">-</Data>
						<Data Name="KeyLength">0</Data>
						<Data Name="ProcessId">0x0</Data>
						<Data Name="ProcessName">-</Data>
						<Data Name="IpAddress">103.151.140.135</Data>
						<Data Name="IpPort">0</Data>
					</EventData>
				</Event>`,
		},
		{
			name: "Tags with dots",
			xml: `<findToFileResponse xmlns="xmlapi_1.0">
					<sysact.Activity>
						<sessionId>UserActivity:0</sessionId>
						<sessionTime>1701734401632</sessionTime>
						<sessionIpAddress>154.11.169.22</sessionIpAddress>
						<serverIpAddress>154.11.169.22</serverIpAddress>
						<sessionType>SamOss</sessionType>
						<time>1701734401634</time>
						<requestId>UserActivity:0</requestId>
						<username>admin</username>
						<type>operation</type>
						<subType>findToFile</subType>
						<fdn>N/A</fdn>
						<classId>0</classId>
						<displayedName>N/A</displayedName>
						<displayedClassName>N/A</displayedClassName>
						<siteId>N/A</siteId>
						<siteName>N/A</siteName>
						<state>success</state>
						<deploymentState>0</deploymentState>
						<objectFullName>425697578</objectFullName>
						<name>N/A</name>
						<selfAlarmed>false</selfAlarmed>
					</sysact.Activity>
					<sysact.Activity>
						<sessionId>XML_API_client@n</sessionId>
						<sessionTime>1701734407353</sessionTime>
						<sessionIpAddress>154.11.169.54</sessionIpAddress>
						<serverIpAddress>154.11.169.22</serverIpAddress>
						<sessionType>SamOss</sessionType>
						<time>1701734407355</time>
						<requestId>XML_API_client@n</requestId>
						<username>meerkat_nsp</username>
						<type>operation</type>
						<subType>findToFile</subType>
						<fdn>N/A</fdn>
						<classId>0</classId>
						<displayedName>N/A</displayedName>
						<displayedClassName>N/A</displayedClassName>
						<siteId>N/A</siteId>
						<siteName>N/A</siteName>
						<state>success</state>
						<deploymentState>0</deploymentState>
						<objectFullName>425697579</objectFullName>
						<name>N/A</name>
						<selfAlarmed>false</selfAlarmed>
					</sysact.Activity>
				</findToFileResponse>`,
		},
	}

	for _, tt := range tests {

		// v1 legacy
		t.Run(tt.name+" legacy", func(t *testing.T) {
			oArgs := &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.xml, nil
					},
				},
			}
			exprFunc, err := createParseXMLFunction[any](ottl.FunctionContext{}, oArgs)
			if tt.createError != "" {
				require.ErrorContains(t, err, tt.createError)
				return
			}

			require.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			if tt.parseError != "" {
				require.ErrorContains(t, err, tt.parseError)
				return
			}

			assert.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)

			rawMap := resultMap.AsRaw()
			require.NotNil(t, rawMap)

			// re-marshal the result to compare the output
			marshalArgs := &MarshalXMLArguments[any]{
				Target: ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return resultMap, nil
					},
				},
			}
			exprFunc, err = createMarshalXMLFunction[any](ottl.FunctionContext{}, marshalArgs)
			require.NoError(t, err)

			result, err = exprFunc(context.Background(), nil)
			require.NoError(t, err)

			resultXML, ok := result.(string)
			require.True(t, ok)

			// remove new lines and tabs from the expected and actual XML
			tt.xml = strings.ReplaceAll(tt.xml, "\n", "")
			tt.xml = strings.ReplaceAll(tt.xml, "\t", "")

			resultXML = strings.ReplaceAll(resultXML, "\n", "")
			resultXML = strings.ReplaceAll(resultXML, "\t", "")

			assert.Equal(t, tt.xml, resultXML)

		})

		// v2
		t.Run(tt.name, func(t *testing.T) {
			oArgs := &ParseXMLArguments[any]{
				FlattenArrays: ottl.NewTestingOptional[ottl.BoolGetter[any]](ottl.StandardBoolGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.flatten, nil
					},
				}),

				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.xml, nil
					},
				},

				Version: ottl.NewTestingOptional[ottl.IntGetter[any]](ottl.StandardIntGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return int64(2), nil
					},
				}),
			}
			exprFunc, err := createParseXMLFunction[any](ottl.FunctionContext{}, oArgs)
			if tt.createError != "" {
				require.ErrorContains(t, err, tt.createError)
				return
			}

			require.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			if tt.parseError != "" {
				require.ErrorContains(t, err, tt.parseError)
				return
			}

			assert.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)

			rawMap := resultMap.AsRaw()
			require.NotNil(t, rawMap)

			// re-marshal the result to compare the output
			marshalArgs := &MarshalXMLArguments[any]{
				Target: ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return resultMap, nil
					},
				},
				Version: ottl.NewTestingOptional[ottl.IntGetter[any]](ottl.StandardIntGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return int64(2), nil
					},
				}),
			}
			exprFunc, err = createMarshalXMLFunction[any](ottl.FunctionContext{}, marshalArgs)
			require.NoError(t, err)

			result, err = exprFunc(context.Background(), nil)
			require.NoError(t, err)

			resultXML, ok := result.(string)
			require.True(t, ok)

			// remove new lines and tabs from the expected and actual XML
			tt.xml = strings.ReplaceAll(tt.xml, "\n", "")
			tt.xml = strings.ReplaceAll(tt.xml, "\t", "")

			resultXML = strings.ReplaceAll(resultXML, "\n", "")
			resultXML = strings.ReplaceAll(resultXML, "\t", "")

			assert.Equal(t, tt.xml, resultXML)

		})
	}
}
