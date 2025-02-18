// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObfuscateSQL(t *testing.T) {
	expected := `SELECT CONVERT ( NVARCHAR, TODATETIMEOFFSET ( CURRENT_TIMESTAMP, DATEPART ( TZOFFSET, SYSDATETIMEOFFSET ( ) ) ), ? ), CONVERT ( NVARCHAR, TODATETIMEOFFSET ( req.start_time, DATEPART ( TZOFFSET, SYSDATETIMEOFFSET ( ) ) ), ? ), sess.login_name, sess.last_request_start_time, sess.session_id, DB_NAME ( sess.database_id ), sess.status, req.status, SUBSTRING ( qt.text, ( req.statement_start_offset / ? ) + ? ( ( CASE req.statement_end_offset WHEN ? THEN DATALENGTH ( qt.text ) ELSE req.statement_end_offset END - req.statement_start_offset ) / ? ) + ? ), SUBSTRING ( qt.text, ? ), c.client_tcp_port, c.client_net_address, sess.host_name, sess.program_name, sess.is_user_process, req.command, req.blocking_session_id, req.wait_type, req.wait_time, req.last_wait_type, req.wait_resource, req.open_transaction_count, req.transaction_id, req.percent_complete, req.estimated_completion_time, req.cpu_time, req.total_elapsed_time, req.reads, req.writes, req.logical_reads, req.transaction_isolation_level, req.lock_timeout, req.deadlock_priority, req.row_count, req.query_hash, req.query_plan_hash, req.context_info FROM sys.dm_exec_sessions sess INNER JOIN sys.dm_exec_connections c ON sess.session_id = c.session_id INNER JOIN sys.dm_exec_requests req ON c.connection_id = req.connection_id CROSS APPLY sys.dm_exec_sql_text ( req.sql_handle ) qt WHERE sess.status != ?`

	origin := `
	SELECT 
		CONVERT(NVARCHAR, TODATETIMEOFFSET(CURRENT_TIMESTAMP, DATEPART(TZOFFSET, SYSDATETIMEOFFSET())), 126) AS now,
		CONVERT(NVARCHAR, TODATETIMEOFFSET(req.start_time, DATEPART(TZOFFSET, SYSDATETIMEOFFSET())), 126) AS query_start,
		sess.login_name AS user_name,
		sess.last_request_start_time AS last_request_start_time,
		sess.session_id AS id,
		DB_NAME(sess.database_id) AS database_name,
		sess.status AS session_status,
		req.status AS request_status,
		SUBSTRING(qt.text, (req.statement_start_offset / 2) + 1, 
				  ((CASE req.statement_end_offset 
					   WHEN -1 THEN DATALENGTH(qt.text) 
					   ELSE req.statement_end_offset 
				   END - req.statement_start_offset) / 2) + 1) AS statement_text,
		SUBSTRING(qt.text, 1, 500) AS text,
		c.client_tcp_port AS client_port,
		c.client_net_address AS client_address,
		sess.host_name AS host_name,
		sess.program_name AS program_name,
		sess.is_user_process AS is_user_process,
		req.command,
		req.blocking_session_id,
		req.wait_type,
		req.wait_time,
		req.last_wait_type,
		req.wait_resource,
		req.open_transaction_count,
		req.transaction_id,
		req.percent_complete,
		req.estimated_completion_time,
		req.cpu_time,
		req.total_elapsed_time,
		req.reads,
		req.writes,
		req.logical_reads,
		req.transaction_isolation_level,
		req.lock_timeout,
		req.deadlock_priority,
		req.row_count,
		req.query_hash,
		req.query_plan_hash,
		req.context_info
	FROM 
		sys.dm_exec_sessions sess
	INNER JOIN 
		sys.dm_exec_connections c ON sess.session_id = c.session_id
	INNER JOIN 
		sys.dm_exec_requests req ON c.connection_id = req.connection_id
	CROSS APPLY 
		sys.dm_exec_sql_text(req.sql_handle) qt
	WHERE 
		sess.status != 'sleeping';
`
	result, err := obfuscateSQL(origin)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestObfuscateQueryPlan(t *testing.T) {
	expected := `<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Version="1.564" Build="16.0.4150.1"><BatchSequence xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><Batch xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><Statements xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><StmtSimple xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" StatementText="SELECT * from table where value = ?" StatementId="1" StatementCompId="1" StatementType="SELECT" RetrievedFromCache="true" StatementSubTreeCost="0.00164412" StatementEstRows="31.6228" SecurityPolicyApplied="false" StatementOptmLevel="FULL" QueryHash="0x52B3B6CEDE6FC57A" QueryPlanHash="0x99E42039E928E3D9" StatementOptmEarlyAbortReason="GoodEnoughPlanFound" CardinalityEstimationModelVersion="160"><StatementSetOptions xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" QUOTED_IDENTIFIER="true" ARITHABORT="false" CONCAT_NULL_YIELDS_NULL="true" ANSI_NULLS="true" ANSI_PADDING="true" ANSI_WARNINGS="true" NUMERIC_ROUNDABORT="false"></StatementSetOptions><QueryPlan xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" CachedPlanSize="32" CompileTime="7" CompileCPU="7" CompileMemory="288"><MemoryGrantInfo xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" SerialRequiredMemory="0" SerialDesiredMemory="0" GrantedMemory="0" MaxUsedMemory="0"></MemoryGrantInfo><OptimizerHardwareDependentProperties xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" EstimatedAvailableMemoryGrant="162534" EstimatedPagesCached="40633" EstimatedAvailableDegreeOfParallelism="2" MaxCompileMemory="3123128"></OptimizerHardwareDependentProperties><RelOp xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" NodeId="0" PhysicalOp="Nested Loops" LogicalOp="Inner Join" EstimateRows="31.6228" EstimateIO="0" EstimateCPU="0.000132183" AvgRowSize="4035" EstimatedTotalSubtreeCost="0.00164412" Parallel="0" EstimateRebinds="0" EstimateRewinds="0" EstimatedExecutionMode="Row"><OutputList xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_data"></ColumnReference></OutputList><NestedLoops xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Optimized="0"><OuterReferences xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="event_session_address"></ColumnReference></OuterReferences><RelOp xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" NodeId="1" PhysicalOp="Filter" LogicalOp="Filter" EstimateRows="31.6228" EstimateIO="0" EstimateCPU="0.00048" AvgRowSize="4041" EstimatedTotalSubtreeCost="0.00148016" Parallel="0" EstimateRebinds="0" EstimateRewinds="0" EstimatedExecutionMode="Row"><OutputList xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="event_session_address"></ColumnReference><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_data"></ColumnReference></OutputList><Filter xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" StartupExpression="0"><RelOp xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" NodeId="2" PhysicalOp="Table-valued function" LogicalOp="Table-valued function" EstimateRows="1000" EstimateIO="0" EstimateCPU="0.00100016" AvgRowSize="4299" EstimatedTotalSubtreeCost="0.00100016" Parallel="0" EstimateRebinds="0" EstimateRewinds="0" EstimatedExecutionMode="Row"><OutputList xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="event_session_address"></ColumnReference><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_name"></ColumnReference><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_data"></ColumnReference></OutputList><TableValuedFunction xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><DefinedValues xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><DefinedValue xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="event_session_address"></ColumnReference></DefinedValue><DefinedValue xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_name"></ColumnReference></DefinedValue><DefinedValue xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_data"></ColumnReference></DefinedValue></DefinedValues><Object xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]"></Object><ParameterList xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="( ? )"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="( ? )"></Const></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="( ? )"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="( ? )"></Const></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="?"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="?"></Const></ScalarOperator></ParameterList></TableValuedFunction></RelOp><Predicate xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="DM_XE_SESSION_TARGETS. [ target_name ] = N ?"><Compare xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" CompareOp="EQ"><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><Identifier xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="target_name"></ColumnReference></Identifier></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="N ?"></Const></ScalarOperator></Compare></ScalarOperator></Predicate></Filter></RelOp><RelOp xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" NodeId="3" PhysicalOp="Table-valued function" LogicalOp="Table-valued function" EstimateRows="1" EstimateIO="0" EstimateCPU="1.157e-06" AvgRowSize="9" EstimatedTotalSubtreeCost="3.17798e-05" Parallel="0" EstimateRebinds="25.9994" EstimateRewinds="4.62341" EstimatedExecutionMode="Row"><OutputList xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"></OutputList><TableValuedFunction xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><DefinedValues xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"></DefinedValues><Object xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSIONS]"></Object><ParameterList xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="( ? )"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="( ? )"></Const></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="( ? )"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="( ? )"></Const></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="DM_XE_SESSION_TARGETS. [ event_session_address ]"><Identifier xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><ColumnReference xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Table="[DM_XE_SESSION_TARGETS]" Column="event_session_address"></ColumnReference></Identifier></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="( ? )"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="( ? )"></Const></ScalarOperator><ScalarOperator xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ScalarString="N ?"><Const xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" ConstValue="N ?"></Const></ScalarOperator></ParameterList></TableValuedFunction></RelOp></NestedLoops></RelOp></QueryPlan></StmtSimple></Statements></Batch></BatchSequence></ShowPlanXML>`
	origin := `
<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan" Version="1.564" Build="16.0.4150.1">
    <BatchSequence>
        <Batch>
            <Statements>
                <StmtSimple
                        StatementText="SELECT * from table where value = 120"
                        StatementId="1" StatementCompId="1" StatementType="SELECT" RetrievedFromCache="true"
                        StatementSubTreeCost="0.00164412" StatementEstRows="31.6228" SecurityPolicyApplied="false"
                        StatementOptmLevel="FULL" QueryHash="0x52B3B6CEDE6FC57A" QueryPlanHash="0x99E42039E928E3D9"
                        StatementOptmEarlyAbortReason="GoodEnoughPlanFound" CardinalityEstimationModelVersion="160">
                    <StatementSetOptions QUOTED_IDENTIFIER="true" ARITHABORT="false" CONCAT_NULL_YIELDS_NULL="true"
                                         ANSI_NULLS="true" ANSI_PADDING="true" ANSI_WARNINGS="true"
                                         NUMERIC_ROUNDABORT="false"/>
                    <QueryPlan CachedPlanSize="32" CompileTime="7" CompileCPU="7" CompileMemory="288">
                        <MemoryGrantInfo SerialRequiredMemory="0" SerialDesiredMemory="0" GrantedMemory="0"
                                         MaxUsedMemory="0"/>
                        <OptimizerHardwareDependentProperties EstimatedAvailableMemoryGrant="162534"
                                                              EstimatedPagesCached="40633"
                                                              EstimatedAvailableDegreeOfParallelism="2"
                                                              MaxCompileMemory="3123128"/>
                        <RelOp NodeId="0" PhysicalOp="Nested Loops" LogicalOp="Inner Join" EstimateRows="31.6228"
                               EstimateIO="0" EstimateCPU="0.000132183" AvgRowSize="4035"
                               EstimatedTotalSubtreeCost="0.00164412" Parallel="0" EstimateRebinds="0"
                               EstimateRewinds="0" EstimatedExecutionMode="Row">
                            <OutputList>
                                <ColumnReference Table="[DM_XE_SESSION_TARGETS]" Column="target_data"/>
                            </OutputList>
                            <NestedLoops Optimized="0">
                                <OuterReferences>
                                    <ColumnReference Table="[DM_XE_SESSION_TARGETS]" Column="event_session_address"/>
                                </OuterReferences>
                                <RelOp NodeId="1" PhysicalOp="Filter" LogicalOp="Filter" EstimateRows="31.6228"
                                       EstimateIO="0" EstimateCPU="0.00048" AvgRowSize="4041"
                                       EstimatedTotalSubtreeCost="0.00148016" Parallel="0" EstimateRebinds="0"
                                       EstimateRewinds="0" EstimatedExecutionMode="Row">
                                    <OutputList>
                                        <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                         Column="event_session_address"/>
                                        <ColumnReference Table="[DM_XE_SESSION_TARGETS]" Column="target_data"/>
                                    </OutputList>
                                    <Filter StartupExpression="0">
                                        <RelOp NodeId="2" PhysicalOp="Table-valued function"
                                               LogicalOp="Table-valued function" EstimateRows="1000" EstimateIO="0"
                                               EstimateCPU="0.00100016" AvgRowSize="4299"
                                               EstimatedTotalSubtreeCost="0.00100016" Parallel="0" EstimateRebinds="0"
                                               EstimateRewinds="0" EstimatedExecutionMode="Row">
                                            <OutputList>
                                                <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                                 Column="event_session_address"/>
                                                <ColumnReference Table="[DM_XE_SESSION_TARGETS]" Column="target_name"/>
                                                <ColumnReference Table="[DM_XE_SESSION_TARGETS]" Column="target_data"/>
                                            </OutputList>
                                            <TableValuedFunction>
                                                <DefinedValues>
                                                    <DefinedValue>
                                                        <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                                         Column="event_session_address"/>
                                                    </DefinedValue>
                                                    <DefinedValue>
                                                        <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                                         Column="target_name"/>
                                                    </DefinedValue>
                                                    <DefinedValue>
                                                        <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                                         Column="target_data"/>
                                                    </DefinedValue>
                                                </DefinedValues>
                                                <Object Table="[DM_XE_SESSION_TARGETS]"/>
                                                <ParameterList>
                                                    <ScalarOperator ScalarString="(0)">
                                                        <Const ConstValue="(0)"/>
                                                    </ScalarOperator>
                                                    <ScalarOperator ScalarString="(0)">
                                                        <Const ConstValue="(0)"/>
                                                    </ScalarOperator>
                                                    <ScalarOperator ScalarString="NULL">
                                                        <Const ConstValue="NULL"/>
                                                    </ScalarOperator>
                                                </ParameterList>
                                            </TableValuedFunction>
                                        </RelOp>
                                        <Predicate>
                                            <ScalarOperator
                                                    ScalarString="DM_XE_SESSION_TARGETS.[target_name]=N'ring_buffer'">
                                                <Compare CompareOp="EQ">
                                                    <ScalarOperator>
                                                        <Identifier>
                                                            <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                                             Column="target_name"/>
                                                        </Identifier>
                                                    </ScalarOperator>
                                                    <ScalarOperator>
                                                        <Const ConstValue="N'ring_buffer'"/>
                                                    </ScalarOperator>
                                                </Compare>
                                            </ScalarOperator>
                                        </Predicate>
                                    </Filter>
                                </RelOp>
                                <RelOp NodeId="3" PhysicalOp="Table-valued function" LogicalOp="Table-valued function"
                                       EstimateRows="1" EstimateIO="0" EstimateCPU="1.157e-06" AvgRowSize="9"
                                       EstimatedTotalSubtreeCost="3.17798e-05" Parallel="0" EstimateRebinds="25.9994"
                                       EstimateRewinds="4.62341" EstimatedExecutionMode="Row">
                                    <OutputList/>
                                    <TableValuedFunction>
                                        <DefinedValues/>
                                        <Object Table="[DM_XE_SESSIONS]"/>
                                        <ParameterList>
                                            <ScalarOperator ScalarString="(0)">
                                                <Const ConstValue="(0)"/>
                                            </ScalarOperator>
                                            <ScalarOperator ScalarString="(1)">
                                                <Const ConstValue="(1)"/>
                                            </ScalarOperator>
                                            <ScalarOperator
                                                    ScalarString="DM_XE_SESSION_TARGETS.[event_session_address]">
                                                <Identifier>
                                                    <ColumnReference Table="[DM_XE_SESSION_TARGETS]"
                                                                     Column="event_session_address"/>
                                                </Identifier>
                                            </ScalarOperator>
                                            <ScalarOperator ScalarString="(1)">
                                                <Const ConstValue="(1)"/>
                                            </ScalarOperator>
                                            <ScalarOperator ScalarString="N'telemetry_xevents'">
                                                <Const ConstValue="N'telemetry_xevents'"/>
                                            </ScalarOperator>
                                        </ParameterList>
                                    </TableValuedFunction>
                                </RelOp>
                            </NestedLoops>
                        </RelOp>
                    </QueryPlan>
                </StmtSimple>
            </Statements>
        </Batch>
    </BatchSequence>
</ShowPlanXML>
`
	result, err := obfuscateXMLPlan(origin)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestInvalidQueryPlans(t *testing.T) {
	plan := `<ShowPlanXml</ShowPlanXML>`
	result, err := obfuscateXMLPlan(plan)
	assert.Empty(t, result)
	assert.Error(t, err)

	plan = `<ShowPlanXML></ShowPlanXML`
	result, err = obfuscateXMLPlan(plan)
	assert.Empty(t, result)
	assert.Error(t, err)

	plan = `<ShowPlanXML></ShowPlan>`
	result, err = obfuscateXMLPlan(plan)
	assert.Empty(t, result)
	assert.Error(t, err)

	// obfuscate failure, but no error
	plan = `<ShowPlanXML StatementText="[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]/(10000)*(3600)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(10000)/(100)*(60)+[msdb].[dbo].[sysjobhistory].[run_duration] as [sjh].[run_duration]%(100)"></ShowPlanXML>`
	result, err = obfuscateXMLPlan(plan)
	assert.Equal(t, plan, result)
	assert.NoError(t, err)
}

func TestValidQueryPlans(t *testing.T) {
	plan := `<ShowPlanXML value="abc"></ShowPlanXML>`
	_, err := obfuscateXMLPlan(plan)
	assert.NoError(t, err)

	plan = `<ShowPlanXML StatementText=""></ShowPlanXML>`
	_, err = obfuscateXMLPlan(plan)
	assert.NoError(t, err)

	plan = `<ShowPlanXML StatementText="SELECT * FROM table"><!-- comment --></ShowPlanXML>`
	_, err = obfuscateXMLPlan(plan)
	assert.NoError(t, err)
}
