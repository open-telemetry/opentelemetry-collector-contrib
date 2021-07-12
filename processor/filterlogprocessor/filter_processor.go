// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterlogprocessor

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterset"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/model/pdata"
)

type filterLogProcessor struct {
	cfg              *Config
	excludeResources filtermatcher.AttributesMatcher
	logger           *zap.Logger
}

func newFilterLogsProcessor(logger *zap.Logger, cfg *Config) (*filterLogProcessor, error) {

	logger.Info(
		"filterlog: Received config newFilterLogsProcessor",
		zap.Any("config", cfg),
	)

	logger.Info(
		"filterlog: Received config newFilterLogsProcessor",
		zap.Any("ExcludeResources Resources", cfg.Logs.ResourceAttributes),
	)

	excludeResources, err := createLogsMatcher(cfg.Logs.ResourceAttributes)
	if err != nil {
		logger.Error(
			"filterlog: Error creating logs resources matcher",
			zap.Error(err),
		)
		return nil, err
	}

	//logger.Info(
	//	"filterlog: Received config newFilterLogsProcessor",
	//	zap.Any("excludeResources", excludeResources),
	//)

	return &filterLogProcessor{
		cfg:              cfg,
		excludeResources: excludeResources,
		logger:           logger,
	}, nil
}

func createLogsMatcher(excludeResources []filterconfig.Attribute) (filtermatcher.AttributesMatcher, error) {
	// Nothing specified in configuration
	if excludeResources == nil {
		return nil, nil
	}
	var attributeMatcher filtermatcher.AttributesMatcher
	attributeMatcher, err := filtermatcher.NewAttributesMatcher(
		filterset.Config{
			MatchType:    filterset.MatchType("strict"),
		},
		excludeResources,
	)
	if err != nil {
		fmt.Println(fmt.Sprintf("filterLog: createLogsMatcher error %s", err.Error()))
		return attributeMatcher, err
	}
	return attributeMatcher, nil
}

func (flp *filterLogProcessor) ProcessLogs(ctx context.Context, logs pdata.Logs) (pdata.Logs, error) {
	//flp.logger.Info("filterlog: ProcessLogs", zap.Any("context", ctx), zap.Any("logs", logs), zap.Any("log_record_count", logs.LogRecordCount()))
	//rls := logs.ResourceLogs()
	//flp.logger.Info("filterlog: ProcessLogs", zap.Any("rls", rls))
	//for i := 0; i < rls.Len(); i++ {
	//	rs := rls.At(i)
	//	ilss := rs.InstrumentationLibraryLogs()
	//	resource := rs.Resource()
	//	for j := 0; j < ilss.Len(); j++ {
	//		ils := ilss.At(j)
	//		logs := ils.Logs()
	//		library := ils.InstrumentationLibrary()
	//		//for k := 0; k < logs.Len(); k++ {
	//		//	lr := logs.At(k)
	//		//	flp.logger.Info("filterlog: ProcessLogs logrecord", zap.Any("lr", lr))
	//		//	//if flp.skipLog(lr, resource, library) {
	//		//	//	continue
	//		//	//}
	//		//	//
	//		//	//flp.attrProc.Process(lr.Attributes())
	//		//}
	//		logs.RemoveIf(func(m pdata.LogRecord) bool {
	//			dontKeep := flp.skipLog(m, resource, library)
	//			flp.logger.Info("filterlog: Filtering in ProcessLogs ", zap.Any("logRecord", m), zap.Any("body", m.Body()),
	//				zap.Any("attributes", m.Attributes()), zap.Bool("dontKeep", dontKeep),
	//				zap.Any("resource", resource), zap.Any("library", library), zap.Any("exclude", flp.exclude))
	//			return dontKeep
	//		})
	//	}
	//}
	//return logs, nil


	//lol := flp.excludeAttribute.MatchLogRecord(logs)
	//logs.ResourceLogs().RemoveIf()

	//flp.logger.Info("Filtering in ProcessLogs")

	// 2nd implementation
	//logs.ResourceLogs().RemoveIf(func(rm pdata.ResourceLogs) bool {
	//	rm.InstrumentationLibraryLogs().RemoveIf(func(ill pdata.InstrumentationLibraryLogs) bool {
	//		ill.Logs().RemoveIf(func(m pdata.LogRecord) bool {
	//			dontKeep := flp.excludeAttribute.MatchLogRecord(m, rm.Resource(), ill.InstrumentationLibrary())
	//			flp.logger.Info("Filtering in ProcessLogs ", zap.Any("logRecord", m), zap.Any("body", m.Body()), zap.Any("attributes", m.Attributes()), zap.Bool("dontKeep", dontKeep))
	//			return dontKeep
	//		})
	//		//flp.logger.Info("Filter out empty InstrumentationLibraryLogs")
	//		// Filter out empty InstrumentationLibraryLogs
	//		return ill.Logs().Len() == 0
	//	})
	//	//flp.logger.Info("Filter out empty ResourceLogs")
	//	// Filter out empty ResourceLogs
	//	return rm.InstrumentationLibraryLogs().Len() == 0
	//})
	//
	//if logs.ResourceLogs().Len() == 0 {
	//	//flp.logger.Info("logs.ResourceLogs().Len() == 0")
	//	return logs, processorhelper.ErrSkipProcessingData
	//}
	//return logs, nil

	//flp.logger.Info("filterlog: ProcessLogs", zap.Any("context", ctx), zap.Any("logs", logs), zap.Any("log_record_count", logs.LogRecordCount()))
	//rls := logs.ResourceLogs()
	//flp.logger.Info("filterlog: ProcessLogs", zap.Any("rls", rls))
	//for i := 0; i < rls.Len(); i++ {
	//	rs := rls.At(i)
	//	ilss := rs.InstrumentationLibraryLogs()
	//	resource := rs.Resource()
	//	for j := 0; j < ilss.Len(); j++ {
	//		ils := ilss.At(j)
	//		logs := ils.Logs()
	//		library := ils.InstrumentationLibrary()
	//		//for k := 0; k < logs.Len(); k++ {
	//		//	lr := logs.At(k)
	//		//	flp.logger.Info("filterlog: ProcessLogs logrecord", zap.Any("lr", lr))
	//		//	//if flp.skipLog(lr, resource, library) {
	//		//	//	continue
	//		//	//}
	//		//	//
	//		//	//flp.attrProc.Process(lr.Attributes())
	//		//}
	//		logs.RemoveIf(func(m pdata.LogRecord) bool {
	//			dontKeep := flp.skipLog(m, resource, library)
	//			flp.logger.Info("filterlog: Filtering in ProcessLogs ", zap.Any("logRecord", m), zap.Any("body", m.Body()),
	//				zap.Any("attributes", m.Attributes()), zap.Bool("dontKeep", dontKeep),
	//				zap.Any("resource", resource), zap.Any("library", library), zap.Any("exclude", flp.exclude))
	//			return dontKeep
	//		})
	//	}
	//}
	fmt.Println("filterLog: ProcessLogs")

	logs.ResourceLogs().RemoveIf(func(rm pdata.ResourceLogs) bool {
		return flp.shouldSkipLogsForResource(rm.Resource())
	})

	fmt.Println("filterLog: ProcessLogs After Remove")

	if logs.ResourceLogs().Len() == 0 {
		return logs, processorhelper.ErrSkipProcessingData
	}

	return logs, nil
}

// skipLog determines if a log should be processed.
// True is returned when a log should be skipped.
// False is returned when a log should not be skipped.
// The logic determining if a log should be processed is set
// in the attribute configuration with the exclude settings.
//func (flp *filterLogProcessor) skipLog(lr pdata.LogRecord, resource pdata.Resource, library pdata.InstrumentationLibrary) bool {
//	fmt.Println("filterLog: skipLog")
//	if flp.exclude != nil {
//		// A true returned in this case means the log should not be processed.
//		if exclude := flp.exclude.MatchLogRecord(lr, resource, library); exclude {
//			return true
//		}
//	}
//
//	fmt.Println("filterLog: skipLog false")
//
//	return false
//}

func (flp *filterLogProcessor) shouldSkipLogsForResource(resource pdata.Resource) bool {
	fmt.Println("filterLog: shouldSkipLogsForResource")
	resourceAttributes := resource.Attributes()

	if flp.excludeResources != nil {
		matches := flp.excludeResources.Match(resourceAttributes)
		if matches {
			fmt.Println("filterLog: shouldSkipLogsForResource Skipping logs woohoo dance baby!!!!")
			return true
		}
	}

	fmt.Println("filterLog: shouldSkipLogsForResource Not Skipping SadFace!!!!")

	return false
}