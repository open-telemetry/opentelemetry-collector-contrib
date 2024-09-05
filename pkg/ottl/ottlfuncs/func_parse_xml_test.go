// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseXML(t *testing.T) {
	tests := []struct {
		name        string
		flatten     bool
		xml         string
		oArgs       ottl.Arguments
		want        map[string]any
		createError string
		parseError  string
	}{
		{
			name: "Windows Event Log XML",
			xml: `<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'>
					<System>
						<Provider Name='Microsoft-Windows-Security-Auditing'
							Guid='{54849625-5478-4994-a5ba-3e3b0328c30d}' />
						<EventID>4625</EventID>
						<Version>0</Version>
						<Level>0</Level>
						<Task>12544</Task>
						<Opcode>0</Opcode>
						<Keywords>0x8010000000000000</Keywords>
						<TimeCreated SystemTime='2024-08-15T20:03:15.9454501Z' />
						<EventRecordID>57220</EventRecordID>
						<Correlation ActivityID='{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}' />
						<Execution ProcessID='656' ThreadID='4652' />
						<Channel>Security</Channel>
						<Computer>samuel-windows</Computer>
						<Security />
					</System>
					<EventData>
						<Data Name='SubjectUserSid'>S-1-0-0</Data>
						<Data Name='SubjectUserName'>-</Data>
						<Data
							Name='SubjectDomainName'>-</Data>
						<Data Name='SubjectLogonId'>0x0</Data>
						<Data
							Name='TargetUserSid'>S-1-0-0</Data>
						<Data Name='TargetUserName'>Administrator</Data>
						<Data
							Name='TargetDomainName'>domain</Data>
						<Data Name='Status'>0xc000006d</Data>
						<Data
							Name='FailureReason'>%%2313</Data>
						<Data Name='SubStatus'>0xc000006a</Data>
						<Data
							Name='LogonType'>3</Data>
						<Data Name='LogonProcessName'>NtLmSsp </Data>
						<Data
							Name='AuthenticationPackageName'>NTLM</Data>
						<Data Name='WorkstationName'>-</Data>
						<Data
							Name='TransmittedServices'>-</Data>
						<Data Name='LmPackageName'>-</Data>
						<Data
							Name='KeyLength'>0</Data>
						<Data Name='ProcessId'>0x0</Data>
						<Data Name='ProcessName'>-</Data>
						<Data
							Name='IpAddress'>103.151.140.135</Data>
						<Data Name='IpPort'>0</Data>
					</EventData>
				</Event>`,
			flatten: false,
			want: map[string]any{
				"Event": map[string]any{
					"xml_attributes": map[string]any{
						"xmlns": "http://schemas.microsoft.com/win/2004/08/events/event",
					},
					"System": map[string]any{
						"EventID":       "4625",
						"xml_ordering":  "Provider,EventID,Version,Level,Task,Opcode,Keywords,TimeCreated,EventRecordID,Correlation,Execution,Channel,Computer,Security",
						"EventRecordID": "57220",
						"Level":         "0",
						"Channel":       "Security",
						"TimeCreated": map[string]any{
							"xml_attributes": map[string]any{
								"SystemTime": "2024-08-15T20:03:15.9454501Z",
							},
						},
						"Version":  "0",
						"Keywords": "0x8010000000000000",
						"Execution": map[string]any{
							"xml_attributes": map[string]any{
								"ProcessID": "656",
								"ThreadID":  "4652",
							},
						},
						"Provider": map[string]any{
							"xml_attributes": map[string]any{
								"Name": "Microsoft-Windows-Security-Auditing",
								"Guid": "{54849625-5478-4994-a5ba-3e3b0328c30d}",
							},
						},
						"Computer": "samuel-windows",
						"Opcode":   "0",
						"Security": "",
						"Task":     "12544",
						"Correlation": map[string]any{
							"xml_attributes": map[string]any{
								"ActivityID": "{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}",
							},
						},
					},
					"EventData": map[string]any{
						"Data": []any{map[string]any{
							"xml_attributes": map[string]any{
								"Name": "SubjectUserSid",
							},
							"xml_value": "S-1-0-0",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "SubjectUserName",
							},
							"xml_value": "-",
						}, map[string]any{
							"xml_value": "-",
							"xml_attributes": map[string]any{
								"Name": "SubjectDomainName",
							},
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "SubjectLogonId",
							},
							"xml_value": "0x0",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "TargetUserSid",
							},
							"xml_value": "S-1-0-0",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "TargetUserName",
							},
							"xml_value": "Administrator",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "TargetDomainName",
							},
							"xml_value": "domain",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "Status",
							},
							"xml_value": "0xc000006d",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "FailureReason",
							},
							"xml_value": "%%2313",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "SubStatus",
							},
							"xml_value": "0xc000006a",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "LogonType",
							},
							"xml_value": "3",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "LogonProcessName",
							},
							"xml_value": "NtLmSsp",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "AuthenticationPackageName",
							},
							"xml_value": "NTLM",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "WorkstationName",
							},
							"xml_value": "-",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "TransmittedServices",
							},
							"xml_value": "-",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "LmPackageName",
							},
							"xml_value": "-",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "KeyLength",
							},
							"xml_value": "0",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "ProcessId",
							},
							"xml_value": "0x0",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "ProcessName",
							},
							"xml_value": "-",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "IpAddress",
							},
							"xml_value": "103.151.140.135",
						}, map[string]any{
							"xml_attributes": map[string]any{
								"Name": "IpPort",
							},
							"xml_value": "0",
						}},
						"xml_ordering": "Data.0,Data.1,Data.2,Data.3,Data.4,Data.5,Data.6,Data.7,Data.8,Data.9,Data.10,Data.11,Data.12,Data.13,Data.14,Data.15,Data.16,Data.17,Data.18,Data.19,Data.20",
					},
					"xml_ordering": "System,EventData",
				},
			},
		},
		{
			name:    "Windows Event Log XML, flattened arrays",
			flatten: true,
			xml: `<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'>
					<System>
						<Provider Name='Microsoft-Windows-Security-Auditing'
							Guid='{54849625-5478-4994-a5ba-3e3b0328c30d}' />
						<EventID>4625</EventID>
						<Version>0</Version>
						<Level>0</Level>
						<Task>12544</Task>
						<Opcode>0</Opcode>
						<Keywords>0x8010000000000000</Keywords>
						<TimeCreated SystemTime='2024-08-15T20:03:15.9454501Z' />
						<EventRecordID>57220</EventRecordID>
						<Correlation ActivityID='{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}' />
						<Execution ProcessID='656' ThreadID='4652' />
						<Channel>Security</Channel>
						<Computer>samuel-windows</Computer>
						<Security />
					</System>
					<EventData>
						<Data Name='SubjectUserSid'>S-1-0-0</Data>
						<Data Name='SubjectUserName'>-</Data>
						<Data
							Name='SubjectDomainName'>-</Data>
						<Data Name='SubjectLogonId'>0x0</Data>
						<Data
							Name='TargetUserSid'>S-1-0-0</Data>
						<Data Name='TargetUserName'>Administrator</Data>
						<Data
							Name='TargetDomainName'>domain</Data>
						<Data Name='Status'>0xc000006d</Data>
						<Data
							Name='FailureReason'>%%2313</Data>
						<Data Name='SubStatus'>0xc000006a</Data>
						<Data
							Name='LogonType'>3</Data>
						<Data Name='LogonProcessName'>NtLmSsp </Data>
						<Data
							Name='AuthenticationPackageName'>NTLM</Data>
						<Data Name='WorkstationName'>-</Data>
						<Data
							Name='TransmittedServices'>-</Data>
						<Data Name='LmPackageName'>-</Data>
						<Data
							Name='KeyLength'>0</Data>
						<Data Name='ProcessId'>0x0</Data>
						<Data Name='ProcessName'>-</Data>
						<Data
							Name='IpAddress'>103.151.140.135</Data>
						<Data Name='IpPort'>0</Data>
					</EventData>
				</Event>`,
			want: map[string]any{
				"Event": map[string]any{
					"xml_attributes": map[string]any{
						"xmlns": "http://schemas.microsoft.com/win/2004/08/events/event",
					},
					"System": map[string]any{
						"Task": "12544",
						"Execution": map[string]any{
							"xml_attributes": map[string]any{
								"ProcessID": "656",
								"ThreadID":  "4652",
							},
						},
						"xml_ordering": "Provider,EventID,Version,Level,Task,Opcode,Keywords,TimeCreated,EventRecordID,Correlation,Execution,Channel,Computer,Security",
						"Opcode":       "0",
						"Keywords":     "0x8010000000000000",
						"Correlation": map[string]any{
							"xml_attributes": map[string]any{
								"ActivityID": "{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}",
							},
						},
						"EventID": "4625",
						"Version": "0",
						"TimeCreated": map[string]any{
							"xml_attributes": map[string]any{
								"SystemTime": "2024-08-15T20:03:15.9454501Z",
							},
						},
						"Level":         "0",
						"EventRecordID": "57220",
						"Provider": map[string]any{
							"xml_attributes": map[string]any{
								"Name": "Microsoft-Windows-Security-Auditing",
								"Guid": "{54849625-5478-4994-a5ba-3e3b0328c30d}",
							},
						},
						"Security": "",
						"Channel":  "Security",
						"Computer": "samuel-windows",
					},
					"EventData": map[string]any{
						"Data": map[string]any{
							"FailureReason":             "%%2313",
							"LogonType":                 "3",
							"AuthenticationPackageName": "NTLM",
							"IpAddress":                 "103.151.140.135",
							"xml_ordering":              "SubjectUserSid,SubjectUserName,SubjectDomainName,SubjectLogonId,TargetUserSid,TargetUserName,TargetDomainName,Status,FailureReason,SubStatus,LogonType,LogonProcessName,AuthenticationPackageName,WorkstationName,TransmittedServices,LmPackageName,KeyLength,ProcessId,ProcessName,IpAddress,IpPort",
							"SubjectUserSid":            "S-1-0-0",
							"TargetDomainName":          "domain",
							"SubStatus":                 "0xc000006a",
							"WorkstationName":           "-",
							"KeyLength":                 "0",
							"ProcessId":                 "0x0",
							"ProcessName":               "-",
							"SubjectDomainName":         "-",
							"SubjectLogonId":            "0x0",
							"TargetUserName":            "Administrator",
							"LogonProcessName":          "NtLmSsp",
							"LmPackageName":             "-",
							"IpPort":                    "0",
							"xml_flattened_array":       "Name",
							"SubjectUserName":           "-",
							"TargetUserSid":             "S-1-0-0",
							"Status":                    "0xc000006d",
							"TransmittedServices":       "-",
						},
					},
					"xml_ordering": "System,EventData",
				},
			},
		},
		{
			xml: "<Log><User><ID>00001</ID><Name>Joe</Name><Email>joe.smith@example.com</Email></User><Text>User did a thing</Text></Log>",
			want: map[string]any{
				"Log": map[string]any{
					"User": map[string]any{
						"ID":           "00001",
						"Name":         "Joe",
						"Email":        "joe.smith@example.com",
						"xml_ordering": "ID,Name,Email",
					},
					"Text":         "User did a thing",
					"xml_ordering": "User,Text",
				},
			},
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
						  <Text>User fired alert A</Text>
						</Log>`,
			want: map[string]any{
				"Log": map[string]any{
					"User": map[string]any{
						"Name":         "Joe",
						"Email":        "joe.smith@example.com",
						"ID":           "00001",
						"xml_ordering": "ID,Name,Email",
					},
					"Text":         "User fired alert A",
					"xml_ordering": "User,Text",
				},
			},
		},
		{
			xml: `<Log>This record has a collision<User id="0001"/><User id="0002"/></Log>`,
			want: map[string]any{
				"Log": map[string]any{
					"xml_value": "This record has a collision",
					"User": []any{map[string]any{
						"xml_attributes": map[string]any{
							"id": "0001",
						},
					}, map[string]any{
						"xml_attributes": map[string]any{
							"id": "0002",
						},
					}},
					"xml_ordering": "User.0,User.1",
				},
			},
		},
		{
			name: "Multiple tags with the same name",
			xml:  `<Log>This record has a collision<User id="0001"/><User id="0002"/></Log>`,
			want: map[string]any{
				"Log": map[string]any{
					"xml_value": "This record has a collision",
					"User": []any{map[string]any{
						"xml_attributes": map[string]any{
							"id": "0001",
						},
					}, map[string]any{
						"xml_attributes": map[string]any{
							"id": "0002",
						},
					}},
					"xml_ordering": "User.0,User.1",
				},
			},
		},
		{
			name: "Multiple lines of content",
			xml: `<Log>
					This record has multiple lines of
					<User id="0001"/>
					text content
				  </Log>`,
			want: map[string]any{
				"Log": map[string]any{
					"xml_value": "This record has multiple lines oftext content",
					"User": map[string]any{
						"xml_attributes": map[string]any{
							"id": "0001",
						},
					},
				},
			},
		},
		{
			name: "Attribute only element",
			xml:  `<HostInfo hostname="example.com" zone="east-1" cloudprovider="aws" />`,
			want: map[string]any{
				"HostInfo": map[string]any{
					"xml_attributes": map[string]any{
						"hostname":      "example.com",
						"zone":          "east-1",
						"cloudprovider": "aws",
					},
				},
			},
		},
		{
			name: "",
			xml: `
<findToFileResponse xmlns="xmlapi_1.0">
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
        <additionalInfo><![CDATA[<params><param name="fullClassName"><value>sysact.Activity</value></param><param name="fileName"><value>user.activity.test.xml.2</value></param><param name="compress"><value>false</value></param><param name="filter"><value>and&gt;greater name="sessionTime" value="1701733501000"&gt;/greater&gt;less name="sessionTime" value="1701734401000"&gt;/less&gt;/and&gt;</value></param><param name="synchronous"><value>true</value></param><param name="requestIdOrNull"><value>UserActivity:0</value></param><param name="timeout"><value>0</value></param><param name="includeSubclasses"><value>true</value></param><param name="timeStamp"><value>false</value></param><param name="principal"><value>admin</value></param></params>]]></additionalInfo>
        <deploymentState>0</deploymentState>
        <objectFullName>425697578</objectFullName>
        <name>N/A</name>
        <selfAlarmed>false</selfAlarmed>
        <children-Set />
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
        <additionalInfo><![CDATA[<params><param name="fullClassName"><value>mpr.IMALink</value></param><param name="fileName"><value>meerkat_fornifi/inventory/INV_mpr.IMALink_findToFile.xml</value></param><param name="compress"><value>false</value></param><param name="filter"><value>not&gt;equal name="siteId" value="0.0.0.0"&gt;/equal&gt;/not&gt;</value></param><param name="synchronous"><value>true</value></param><param name="requestIdOrNull"><value>XML_API_client@n</value></param><param name="timeout"><value>0</value></param><param name="includeSubclasses"><value>true</value></param><param name="timeStamp"><value>false</value></param><param name="principal"><value>meerkat_nsp</value></param></params>]]></additionalInfo>
        <deploymentState>0</deploymentState>
        <objectFullName>425697579</objectFullName>
        <name>N/A</name>
        <selfAlarmed>false</selfAlarmed>
        <children-Set />
    </sysact.Activity>
</findToFileResponse>`,
			want: map[string]any{
				"findToFileResponse": map[string]any{
					"xml_attributes": map[string]any{
						"xmlns": "xmlapi_1.0",
					},
					"sysact.Activity": []any{
						map[string]any{
							"state":              "success",
							"serverIpAddress":    "154.11.169.22",
							"time":               "1701734401634",
							"username":           "admin",
							"classId":            "0",
							"sessionId":          "UserActivity:0",
							"siteId":             "N/A",
							"displayedName":      "N/A",
							"xml_ordering":       "sessionId,sessionTime,sessionIpAddress,serverIpAddress,sessionType,time,requestId,username,type,subType,fdn,classId,displayedName,displayedClassName,siteId,siteName,state,additionalInfo,deploymentState,objectFullName,name,selfAlarmed,children-Set",
							"objectFullName":     "425697578",
							"sessionTime":        "1701734401632",
							"type":               "operation",
							"sessionType":        "SamOss",
							"displayedClassName": "N/A",
							"siteName":           "N/A",
							"fdn":                "N/A",
							"deploymentState":    "0",
							"name":               "N/A",
							"subType":            "findToFile",
							"selfAlarmed":        "false",
							"additionalInfo":     "<params><param name=\"fullClassName\"><value>sysact.Activity</value></param><param name=\"fileName\"><value>user.activity.test.xml.2</value></param><param name=\"compress\"><value>false</value></param><param name=\"filter\"><value>and&gt;greater name=\"sessionTime\" value=\"1701733501000\"&gt;/greater&gt;less name=\"sessionTime\" value=\"1701734401000\"&gt;/less&gt;/and&gt;</value></param><param name=\"synchronous\"><value>true</value></param><param name=\"requestIdOrNull\"><value>UserActivity:0</value></param><param name=\"timeout\"><value>0</value></param><param name=\"includeSubclasses\"><value>true</value></param><param name=\"timeStamp\"><value>false</value></param><param name=\"principal\"><value>admin</value></param></params>",
							"sessionIpAddress":   "154.11.169.22",
							"children-Set":       "",
							"requestId":          "UserActivity:0",
						},
						map[string]any{
							"children-Set":       "",
							"sessionIpAddress":   "154.11.169.54",
							"classId":            "0",
							"selfAlarmed":        "false",
							"state":              "success",
							"subType":            "findToFile",
							"siteId":             "N/A",
							"deploymentState":    "0",
							"sessionType":        "SamOss",
							"xml_ordering":       "sessionId,sessionTime,sessionIpAddress,serverIpAddress,sessionType,time,requestId,username,type,subType,fdn,classId,displayedName,displayedClassName,siteId,siteName,state,additionalInfo,deploymentState,objectFullName,name,selfAlarmed,children-Set",
							"sessionId":          "XML_API_client@n",
							"time":               "1701734407355",
							"displayedName":      "N/A",
							"additionalInfo":     "<params><param name=\"fullClassName\"><value>mpr.IMALink</value></param><param name=\"fileName\"><value>meerkat_fornifi/inventory/INV_mpr.IMALink_findToFile.xml</value></param><param name=\"compress\"><value>false</value></param><param name=\"filter\"><value>not&gt;equal name=\"siteId\" value=\"0.0.0.0\"&gt;/equal&gt;/not&gt;</value></param><param name=\"synchronous\"><value>true</value></param><param name=\"requestIdOrNull\"><value>XML_API_client@n</value></param><param name=\"timeout\"><value>0</value></param><param name=\"includeSubclasses\"><value>true</value></param><param name=\"timeStamp\"><value>false</value></param><param name=\"principal\"><value>meerkat_nsp</value></param></params>",
							"sessionTime":        "1701734407353",
							"requestId":          "XML_API_client@n",
							"username":           "meerkat_nsp",
							"objectFullName":     "425697579",
							"displayedClassName": "N/A",
							"fdn":                "N/A",
							"type":               "operation",
							"name":               "N/A",
							"serverIpAddress":    "154.11.169.22",
							"siteName":           "N/A",
						},
					},
					"xml_ordering": "sysact.Activity.0,sysact.Activity.1",
				},
			},
		},
		{
			name: "example",
			xml: `<Log>
  <User id="00001">
   Sam
  </User>
  <User id="00002">
   Bob
  </User>
  <User id="00003">
   Alice
  </User>
</Log>`,
		},
	}

	for _, tt := range tests {
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
			// require.Equal(t, tt.want, rawMap)
			t.Logf("Result: %s", formatMap(rawMap))

			// re-marshal the result to compare the output

			// marshalArgs := &MarshalXMLArguments[any]{
			// 	Target: ottl.StandardPMapGetter[any]{
			// 		Getter: func(_ context.Context, _ any) (any, error) {
			// 			return resultMap, nil
			// 		},
			// 	},
			// }
			// exprFunc, err = createMarshalXMLFunction[any](ottl.FunctionContext{}, marshalArgs)
			// require.NoError(t, err)

			// result, err = exprFunc(context.Background(), nil)
			// require.NoError(t, err)

			// resultXML, ok := result.(string)
			// require.True(t, ok)

			// // We'd like to compare the XML output, but variations in whitespace, quoting, and most
			// // importantly, the use of closing tags vs. self-closing tags, make this difficult.
			// // Instead, we'll parse the marshaled XML and compare the resulting map.

			// oArgs.Target = ottl.StandardStringGetter[any]{
			// 	Getter: func(_ context.Context, _ any) (any, error) {
			// 		return resultXML, nil
			// 	},
			// }
			// exprFunc, err = createParseXMLFunction[any](ottl.FunctionContext{}, oArgs)
			// require.NoError(t, err)
			// result, err = exprFunc(context.Background(), nil)
			// require.NoError(t, err)
			// resultMap, ok = result.(pcommon.Map)
			// require.True(t, ok)

			// rawMap = resultMap.AsRaw()
			// require.NotNil(t, rawMap)
			// require.Equal(t, tt.want, rawMap)

		})
	}
}

func formatMap(m map[string]any) string {
	var b strings.Builder
	b.WriteString("map[string]any{\n")
	for key, value := range m {
		fmt.Fprintf(&b, "\t%q: %s,\n", key, formatValue(value))
	}
	b.WriteString("}")
	return b.String()
}

func formatValue(value any) string {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return fmt.Sprintf("%q", value)
	case reflect.Map:
		if mapValue, ok := value.(map[string]any); ok {
			return formatMap(mapValue)
		}
	case reflect.Slice:
		b := strings.Builder{}
		b.WriteString("[]any{")
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(formatValue(v.Index(i).Interface()))
		}
		b.WriteString("}")
		return b.String()
	}
	return fmt.Sprintf("%v", value)
}
